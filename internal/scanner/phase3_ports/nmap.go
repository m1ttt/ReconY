package phase3

import (
	"context"
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"reconx/internal/engine"
	"reconx/internal/models"
	"reconx/internal/tools"
)

// NmapRunner wraps nmap for port scanning.
type NmapRunner struct{}

func (n *NmapRunner) Name() string         { return "nmap" }
func (n *NmapRunner) Phase() engine.PhaseID { return engine.PhasePorts }
func (n *NmapRunner) Check() error          { return tools.CheckBinary("nmap") }

func (n *NmapRunner) Run(ctx context.Context, input *engine.PhaseInput, sink engine.ResultSink) error {
	// Collect alive subdomains as targets
	var targets []string
	for _, sub := range input.Subdomains {
		if sub.IsAlive {
			targets = append(targets, sub.Hostname)
		}
	}
	if len(targets) == 0 {
		// Fallback: scan all subdomains
		for _, sub := range input.Subdomains {
			targets = append(targets, sub.Hostname)
		}
	}
	if len(targets) == 0 {
		targets = []string{input.Workspace.Domain}
	}

	sink.LogLine(ctx, "stdout", fmt.Sprintf("Scanning %d targets with nmap", len(targets)))

	// Write targets to temp file
	tmpDir := os.TempDir()
	targetFile := filepath.Join(tmpDir, fmt.Sprintf("reconx-nmap-targets-%s.txt", input.ScanJobID))
	defer os.Remove(targetFile)
	os.WriteFile(targetFile, []byte(strings.Join(targets, "\n")), 0644)

	// Output XML file
	xmlFile := filepath.Join(tmpDir, fmt.Sprintf("reconx-nmap-output-%s.xml", input.ScanJobID))
	defer os.Remove(xmlFile)

	// Build nmap args
	tc := input.Config.GetToolConfig("nmap")
	args := []string{"-iL", targetFile, "-oX", xmlFile}

	// Scan type
	scanType := "-sT"
	if opts := tc.Options; opts != nil {
		if v, ok := opts["scan_type"].(string); ok {
			scanType = v
		}
	}
	args = append(args, scanType)

	// Timing
	timing := "-T4"
	if opts := tc.Options; opts != nil {
		if v, ok := opts["timing"].(string); ok {
			timing = v
		}
	}
	args = append(args, timing)

	// Ports
	ports := "top-1000"
	if opts := tc.Options; opts != nil {
		if v, ok := opts["ports"].(string); ok {
			ports = v
		}
	}
	switch ports {
	case "top-100":
		args = append(args, "--top-ports", "100")
	case "top-1000":
		args = append(args, "--top-ports", "1000")
	case "full":
		args = append(args, "-p-")
	default:
		args = append(args, "-p", ports)
	}

	// Skip host discovery (some hosts block ping) + service detection
	args = append(args, "-Pn", "-sV", "--version-intensity", "5")

	args = append(args, tc.ExtraArgs...)

	result, err := tools.RunToolWithProxy(ctx, "nmap", args, input.ProxyURL, func(stream, line string) {
		sink.LogLine(ctx, stream, line)
	})
	if err != nil {
		return fmt.Errorf("running nmap: %w", err)
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("nmap exited with code %d", result.ExitCode)
	}

	// Parse XML output
	return parseNmapXML(ctx, xmlFile, input, sink)
}

// Nmap XML structures
type nmapRun struct {
	XMLName xml.Name   `xml:"nmaprun"`
	Hosts   []nmapHost `xml:"host"`
}

type nmapHost struct {
	Address  []nmapAddr `xml:"address"`
	Hostname []struct {
		Name string `xml:"name,attr"`
	} `xml:"hostnames>hostname"`
	Ports []nmapPort `xml:"ports>port"`
}

type nmapAddr struct {
	Addr     string `xml:"addr,attr"`
	AddrType string `xml:"addrtype,attr"`
}

type nmapPort struct {
	Protocol string `xml:"protocol,attr"`
	PortID   int    `xml:"portid,attr"`
	State    struct {
		State string `xml:"state,attr"`
	} `xml:"state"`
	Service struct {
		Name    string `xml:"name,attr"`
		Product string `xml:"product,attr"`
		Version string `xml:"version,attr"`
		Extra   string `xml:"extrainfo,attr"`
	} `xml:"service"`
}

func parseNmapXML(ctx context.Context, xmlFile string, input *engine.PhaseInput, sink engine.ResultSink) error {
	data, err := os.ReadFile(xmlFile)
	if err != nil {
		return fmt.Errorf("reading nmap output: %w", err)
	}

	var run nmapRun
	if err := xml.Unmarshal(data, &run); err != nil {
		return fmt.Errorf("parsing nmap XML: %w", err)
	}

	for _, host := range run.Hosts {
		var ip string
		for _, addr := range host.Address {
			if addr.AddrType == "ipv4" || addr.AddrType == "ipv6" {
				ip = addr.Addr
				break
			}
		}
		if ip == "" {
			continue
		}

		// Find matching subdomain
		var subdomainID *string
		hostname := ""
		if len(host.Hostname) > 0 {
			hostname = host.Hostname[0].Name
		}
		for _, sub := range input.Subdomains {
			if sub.Hostname == hostname {
				subdomainID = &sub.ID
				break
			}
		}

		for _, port := range host.Ports {
			var service, version, banner *string
			if port.Service.Name != "" {
				s := port.Service.Name
				service = &s
			}
			if port.Service.Version != "" {
				v := port.Service.Product
				if port.Service.Version != "" {
					v += " " + port.Service.Version
				}
				version = &v
			}
			if port.Service.Extra != "" {
				b := port.Service.Extra
				banner = &b
			}

			sink.LogLine(ctx, "stdout", fmt.Sprintf("%s:%d/%s %s %s",
				ip, port.PortID, port.Protocol, port.State.State,
				strconv.Quote(stringVal(service))))

			sink.AddPort(ctx, &models.Port{
				SubdomainID: subdomainID,
				IPAddress:   ip,
				Port:        port.PortID,
				Protocol:    port.Protocol,
				State:       port.State.State,
				Service:     service,
				Version:     version,
				Banner:      banner,
			})
		}
	}

	return nil
}

func stringVal(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
