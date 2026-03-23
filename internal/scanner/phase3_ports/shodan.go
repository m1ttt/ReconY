package phase3

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"time"

	"reconx/internal/engine"
	"reconx/internal/httpkit"
	"reconx/internal/models"
)

// ShodanRunner queries the Shodan API for host information.
type ShodanRunner struct{}

func (s *ShodanRunner) Name() string         { return "shodan" }
func (s *ShodanRunner) Phase() engine.PhaseID { return engine.PhasePorts }
func (s *ShodanRunner) Check() error {
	// Shodan doesn't need a binary, just an API key
	return nil
}

func (s *ShodanRunner) Run(ctx context.Context, input *engine.PhaseInput, sink engine.ResultSink) error {
	apiKey := input.Config.APIKeys.Shodan
	if apiKey == "" {
		sink.LogLine(ctx, "stderr", "Shodan API key not configured, skipping")
		return nil
	}

	// Resolve target IPs
	var ips []string
	seen := make(map[string]bool)
	for _, sub := range input.Subdomains {
		if !sub.IsAlive {
			continue
		}
		addrs, err := net.LookupHost(sub.Hostname)
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			if !seen[addr] {
				seen[addr] = true
				ips = append(ips, addr)
			}
		}
	}

	if len(ips) == 0 {
		// Fallback to main domain
		addrs, _ := net.LookupHost(input.Workspace.Domain)
		ips = addrs
	}

	sink.LogLine(ctx, "stdout", fmt.Sprintf("Querying Shodan for %d IPs", len(ips)))

	client := httpkit.NewClient(input.Config)

	for _, ip := range ips {
		if err := ctx.Err(); err != nil {
			return err
		}

		if err := s.queryHost(ctx, client, apiKey, ip, input, sink); err != nil {
			sink.LogLine(ctx, "stderr", fmt.Sprintf("Shodan query for %s failed: %v", ip, err))
		}

		// Rate limit: Shodan free tier is 1 req/sec
		time.Sleep(1100 * time.Millisecond)
	}

	return nil
}

func (s *ShodanRunner) queryHost(ctx context.Context, client *httpkit.Client, apiKey, ip string, input *engine.PhaseInput, sink engine.ResultSink) error {
	url := fmt.Sprintf("https://api.shodan.io/shodan/host/%s?key=%s", ip, apiKey)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		sink.LogLine(ctx, "stdout", fmt.Sprintf("%s: no Shodan data", ip))
		return nil
	}
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("shodan API %d: %s", resp.StatusCode, string(body))
	}

	var result shodanHost
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return err
	}

	// Find matching subdomain
	var subdomainID *string
	for _, sub := range input.Subdomains {
		if sub.IPAddresses != nil {
			var addrs []string
			json.Unmarshal([]byte(*sub.IPAddresses), &addrs)
			for _, a := range addrs {
				if a == ip {
					subdomainID = &sub.ID
					break
				}
			}
		}
	}

	for _, svc := range result.Data {
		service := svc.Product
		if service == "" {
			service = svc.Transport
		}
		var version, banner *string
		if svc.Version != "" {
			v := svc.Version
			version = &v
		}
		if svc.Banner != "" {
			b := svc.Banner
			if len(b) > 500 {
				b = b[:500]
			}
			banner = &b
		}

		sink.LogLine(ctx, "stdout", fmt.Sprintf("%s:%d %s %s", ip, svc.Port, service, strconv.Quote(stringVal(version))))

		svcPtr := &service
		sink.AddPort(ctx, &models.Port{
			SubdomainID: subdomainID,
			IPAddress:   ip,
			Port:        svc.Port,
			Protocol:    svc.Transport,
			State:       "open",
			Service:     svcPtr,
			Version:     version,
			Banner:      banner,
		})
	}

	return nil
}

type shodanHost struct {
	IP   string       `json:"ip_str"`
	Data []shodanData `json:"data"`
}

type shodanData struct {
	Port      int    `json:"port"`
	Transport string `json:"transport"`
	Product   string `json:"product"`
	Version   string `json:"version"`
	Banner    string `json:"data"`
}
