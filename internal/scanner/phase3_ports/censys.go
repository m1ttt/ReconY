package phase3

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	"reconx/internal/engine"
	"reconx/internal/httpkit"
	"reconx/internal/models"
)

// CensysRunner queries the Censys API for host information.
type CensysRunner struct{}

func (c *CensysRunner) Name() string         { return "censys" }
func (c *CensysRunner) Phase() engine.PhaseID { return engine.PhasePorts }
func (c *CensysRunner) Check() error          { return nil }

func (c *CensysRunner) Run(ctx context.Context, input *engine.PhaseInput, sink engine.ResultSink) error {
	apiID := input.Config.APIKeys.CensysID
	apiSecret := input.Config.APIKeys.CensysSecret
	if apiID == "" || apiSecret == "" {
		sink.LogLine(ctx, "stderr", "Censys API credentials not configured, skipping")
		return nil
	}

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
		addrs, _ := net.LookupHost(input.Workspace.Domain)
		ips = addrs
	}

	sink.LogLine(ctx, "stdout", fmt.Sprintf("Querying Censys for %d IPs", len(ips)))
	client := httpkit.NewClient(input.Config)

	for _, ip := range ips {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := c.queryHost(ctx, client, apiID, apiSecret, ip, input, sink); err != nil {
			sink.LogLine(ctx, "stderr", fmt.Sprintf("Censys %s: %v", ip, err))
		}
		time.Sleep(500 * time.Millisecond)
	}

	return nil
}

func (c *CensysRunner) queryHost(ctx context.Context, client *httpkit.Client, apiID, apiSecret, ip string, input *engine.PhaseInput, sink engine.ResultSink) error {
	url := fmt.Sprintf("https://search.censys.io/api/v2/hosts/%s", ip)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}
	req.SetBasicAuth(apiID, apiSecret)

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return nil
	}
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("censys %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Result struct {
			IP       string `json:"ip"`
			Services []struct {
				Port            int    `json:"port"`
				TransportProto  string `json:"transport_protocol"`
				ServiceName     string `json:"service_name"`
				Banner          string `json:"banner"`
				Software        []struct {
					Product string `json:"product"`
					Version string `json:"version"`
				} `json:"software"`
			} `json:"services"`
		} `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return err
	}

	for _, svc := range result.Result.Services {
		var service, version, banner *string
		if svc.ServiceName != "" {
			s := svc.ServiceName
			service = &s
		}
		if svc.Banner != "" {
			b := svc.Banner
			if len(b) > 500 {
				b = b[:500]
			}
			banner = &b
		}
		if len(svc.Software) > 0 {
			v := svc.Software[0].Product
			if svc.Software[0].Version != "" {
				v += " " + svc.Software[0].Version
			}
			version = &v
		}

		proto := "tcp"
		if svc.TransportProto != "" {
			proto = svc.TransportProto
		}

		sink.AddPort(ctx, &models.Port{
			IPAddress: ip,
			Port:      svc.Port,
			Protocol:  proto,
			State:     "open",
			Service:   service,
			Version:   version,
			Banner:    banner,
		})
	}

	return nil
}
