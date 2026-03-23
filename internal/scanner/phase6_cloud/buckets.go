package phase6

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	"reconx/internal/engine"
	"reconx/internal/httpkit"
	"reconx/internal/models"
)

// BucketEnumRunner discovers exposed cloud storage buckets.
type BucketEnumRunner struct{}

func (b *BucketEnumRunner) Name() string         { return "bucket_enum" }
func (b *BucketEnumRunner) Phase() engine.PhaseID { return engine.PhaseCloud }
func (b *BucketEnumRunner) Check() error          { return nil }

func (b *BucketEnumRunner) Run(ctx context.Context, input *engine.PhaseInput, sink engine.ResultSink) error {
	domain := input.Workspace.Domain
	baseName := strings.Split(domain, ".")[0]

	// Generate bucket name permutations
	permutations := generateBucketNames(baseName, domain)
	sink.LogLine(ctx, "stdout", fmt.Sprintf("Checking %d bucket permutations", len(permutations)))

	client := httpkit.NewClient(input.Config)

	// Check S3
	for _, name := range permutations {
		if err := ctx.Err(); err != nil {
			return err
		}
		b.checkS3(ctx, client, name, sink)
	}

	// Check Azure Blob
	for _, name := range permutations {
		if err := ctx.Err(); err != nil {
			return err
		}
		b.checkAzure(ctx, client, name, sink)
	}

	// Check GCP
	for _, name := range permutations {
		if err := ctx.Err(); err != nil {
			return err
		}
		b.checkGCP(ctx, client, name, sink)
	}

	return nil
}

func (b *BucketEnumRunner) checkS3(ctx context.Context, client *httpkit.Client, name string, sink engine.ResultSink) {
	url := fmt.Sprintf("https://%s.s3.amazonaws.com", name)
	b.checkBucket(ctx, client, url, name, "aws", "bucket", sink)
}

func (b *BucketEnumRunner) checkAzure(ctx context.Context, client *httpkit.Client, name string, sink engine.ResultSink) {
	url := fmt.Sprintf("https://%s.blob.core.windows.net", name)
	b.checkBucket(ctx, client, url, name, "azure", "blob_container", sink)
}

func (b *BucketEnumRunner) checkGCP(ctx context.Context, client *httpkit.Client, name string, sink engine.ResultSink) {
	url := fmt.Sprintf("https://storage.googleapis.com/%s", name)
	b.checkBucket(ctx, client, url, name, "gcp", "bucket", sink)
}

func (b *BucketEnumRunner) checkBucket(ctx context.Context, client *httpkit.Client, url, name, provider, assetType string, sink engine.ResultSink) {
	req, err := http.NewRequestWithContext(ctx, "HEAD", url, nil)
	if err != nil {
		return
	}

	resp, err := client.Do(req)
	if err != nil {
		return
	}
	defer io.Copy(io.Discard, resp.Body)
	resp.Body.Close()

	// Interesting status codes
	switch resp.StatusCode {
	case 200:
		// Public bucket!
		sink.LogLine(ctx, "stdout", fmt.Sprintf("[!] PUBLIC %s bucket: %s", provider, name))
		urlPtr := url
		sink.AddCloudAsset(ctx, &models.CloudAsset{
			Provider:  provider,
			AssetType: assetType,
			Name:      name,
			URL:       &urlPtr,
			IsPublic:  true,
		})
	case 403:
		// Exists but private
		sink.LogLine(ctx, "stdout", fmt.Sprintf("[*] Private %s bucket: %s", provider, name))
		urlPtr := url
		sink.AddCloudAsset(ctx, &models.CloudAsset{
			Provider:  provider,
			AssetType: assetType,
			Name:      name,
			URL:       &urlPtr,
			IsPublic:  false,
		})
	}
	// 404 = doesn't exist, skip
}

func generateBucketNames(baseName, domain string) []string {
	suffixes := []string{
		"", "-dev", "-staging", "-prod", "-production", "-backup", "-backups",
		"-data", "-assets", "-media", "-uploads", "-static", "-files",
		"-logs", "-test", "-internal", "-private", "-public",
		"-cdn", "-images", "-docs", "-db", "-database",
	}

	var names []string
	seen := make(map[string]bool)

	bases := []string{baseName, domain, strings.ReplaceAll(domain, ".", "-")}

	for _, base := range bases {
		for _, suffix := range suffixes {
			name := base + suffix
			if !seen[name] {
				seen[name] = true
				names = append(names, name)
			}
		}
	}

	return names
}
