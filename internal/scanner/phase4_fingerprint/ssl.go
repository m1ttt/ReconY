package phase4

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"time"

	"reconx/internal/engine"
	"reconx/internal/models"
)

// SSLAnalyzeRunner checks TLS/SSL configuration.
type SSLAnalyzeRunner struct{}

func (s *SSLAnalyzeRunner) Name() string         { return "ssl_analyze" }
func (s *SSLAnalyzeRunner) Phase() engine.PhaseID { return engine.PhaseFingerprint }
func (s *SSLAnalyzeRunner) Check() error          { return nil }

func (s *SSLAnalyzeRunner) Run(ctx context.Context, input *engine.PhaseInput, sink engine.ResultSink) error {
	var targets []string
	for _, sub := range input.Subdomains {
		if sub.IsAlive {
			targets = append(targets, sub.Hostname)
		}
	}
	if len(targets) == 0 {
		for _, sub := range input.Subdomains {
			targets = append(targets, sub.Hostname)
		}
	}
	if len(targets) == 0 {
		targets = []string{input.Workspace.Domain}
	}

	sink.LogLine(ctx, "stdout", fmt.Sprintf("Analyzing SSL/TLS on %d targets", len(targets)))

	for _, host := range targets {
		if err := ctx.Err(); err != nil {
			return err
		}
		s.analyzeHost(ctx, host, input, sink)
	}

	return nil
}

func (s *SSLAnalyzeRunner) analyzeHost(ctx context.Context, host string, input *engine.PhaseInput, sink engine.ResultSink) {
	dialer := &net.Dialer{Timeout: 5 * time.Second}
	conn, err := tls.DialWithDialer(dialer, "tcp", host+":443", &tls.Config{
		InsecureSkipVerify: true,
	})
	if err != nil {
		sink.LogLine(ctx, "stderr", fmt.Sprintf("%s: TLS connection failed: %v", host, err))
		return
	}
	defer conn.Close()

	state := conn.ConnectionState()

	details := map[string]any{
		"version":     tlsVersionName(state.Version),
		"cipher":      tls.CipherSuiteName(state.CipherSuite),
		"server_name": state.ServerName,
	}

	// Check certificates
	if len(state.PeerCertificates) > 0 {
		cert := state.PeerCertificates[0]
		details["subject"] = cert.Subject.CommonName
		details["issuer"] = cert.Issuer.CommonName
		details["not_before"] = cert.NotBefore.Format(time.RFC3339)
		details["not_after"] = cert.NotAfter.Format(time.RFC3339)
		details["sans"] = cert.DNSNames

		// Check if expired
		now := time.Now()
		if now.After(cert.NotAfter) {
			details["expired"] = true
		}
		if now.Before(cert.NotBefore) {
			details["not_yet_valid"] = true
		}

		// Detect hosting from cert
		for _, san := range cert.DNSNames {
			lower := strings.ToLower(san)
			if strings.Contains(lower, "vercel") || strings.Contains(lower, "netlify") ||
				strings.Contains(lower, "herokuapp") || strings.Contains(lower, "cloudfront") {
				details["hosting_hint"] = san
			}
		}
	}

	grade := gradeSSL(state)
	details["grade"] = grade

	detailsJSON, _ := json.Marshal(details)
	detailsStr := string(detailsJSON)
	gradeStr := grade

	var subdomainID *string
	for _, sub := range input.Subdomains {
		if sub.Hostname == host {
			subdomainID = &sub.ID
			break
		}
	}

	sink.LogLine(ctx, "stdout", fmt.Sprintf("%s: SSL grade %s, %s, %s",
		host, grade, details["version"], details["cipher"]))

	sink.AddSiteClassification(ctx, &models.SiteClassification{
		SubdomainID: subdomainID,
		URL:         "https://" + host,
		SiteType:    models.SiteTypeUnknown,
		SSLGrade:    &gradeStr,
		SSLDetails:  &detailsStr,
	})
}

func tlsVersionName(v uint16) string {
	switch v {
	case tls.VersionTLS13:
		return "TLS 1.3"
	case tls.VersionTLS12:
		return "TLS 1.2"
	case tls.VersionTLS11:
		return "TLS 1.1"
	case tls.VersionTLS10:
		return "TLS 1.0"
	default:
		return fmt.Sprintf("0x%04x", v)
	}
}

func gradeSSL(state tls.ConnectionState) string {
	// Simple grading based on TLS version and cipher
	if state.Version >= tls.VersionTLS13 {
		return "A+"
	}
	if state.Version >= tls.VersionTLS12 {
		return "A"
	}
	if state.Version >= tls.VersionTLS11 {
		return "B"
	}
	return "F"
}
