package phase1

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"strings"

	"reconx/internal/engine"
	"reconx/internal/models"
	"reconx/internal/tools"
)

// WhoisRunner performs WHOIS lookups on the target domain.
type WhoisRunner struct{}

func (w *WhoisRunner) Name() string        { return "whois" }
func (w *WhoisRunner) Phase() engine.PhaseID { return engine.PhasePassive }
func (w *WhoisRunner) Check() error         { return tools.CheckBinary("whois") }

func (w *WhoisRunner) Run(ctx context.Context, input *engine.PhaseInput, sink engine.ResultSink) error {
	domain := input.Workspace.Domain

	// Run whois command
	var rawOutput strings.Builder
	result, err := tools.RunToolWithProxy(ctx, "whois", []string{domain}, input.ProxyURL, func(stream, line string) {
		sink.LogLine(ctx, stream, line)
		if stream == "stdout" {
			rawOutput.WriteString(line + "\n")
		}
	})
	if err != nil {
		return fmt.Errorf("running whois: %w", err)
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("whois exited with code %d", result.ExitCode)
	}

	raw := rawOutput.String()
	record := &models.WhoisRecord{
		Domain: domain,
		Raw:    &raw,
	}

	// Parse WHOIS output
	record.Registrar = extractField(raw, "Registrar:", "Registrar Name:")
	record.Org = extractField(raw, "Registrant Organization:", "OrgName:", "org-name:")
	record.Country = extractField(raw, "Registrant Country:", "Country:")
	record.CreationDate = extractField(raw, "Creation Date:", "created:")
	record.ExpiryDate = extractField(raw, "Registry Expiry Date:", "Expiry Date:", "expires:")

	// Parse name servers
	nameServers := extractMultiple(raw, "Name Server:", "nserver:")
	if len(nameServers) > 0 {
		nsJSON, _ := json.Marshal(nameServers)
		nsStr := string(nsJSON)
		record.NameServers = &nsStr
	}

	// Try to get ASN info via DNS lookup of the IP
	ips, err := net.LookupHost(domain)
	if err == nil && len(ips) > 0 {
		asnInfo := lookupASN(ctx, ips[0], sink)
		if asnInfo != nil {
			record.ASN = asnInfo.ASN
			record.ASNOrg = asnInfo.Org
			record.ASNCIDR = asnInfo.CIDR
		}
	}

	return sink.AddWhoisRecord(ctx, record)
}

type asnResult struct {
	ASN  *string
	Org  *string
	CIDR *string
}

func lookupASN(ctx context.Context, ip string, sink engine.ResultSink) *asnResult {
	// Use Team Cymru DNS-based ASN lookup
	parts := strings.Split(ip, ".")
	if len(parts) != 4 {
		return nil
	}
	// Reverse the IP for DNS query
	reversed := fmt.Sprintf("%s.%s.%s.%s.origin.asn.cymru.com", parts[3], parts[2], parts[1], parts[0])

	txts, err := net.LookupTXT(reversed)
	if err != nil || len(txts) == 0 {
		return nil
	}

	// Format: "ASN | IP/CIDR | CC | Registry | Allocated"
	fields := strings.Split(txts[0], "|")
	if len(fields) < 3 {
		return nil
	}

	result := &asnResult{}
	asn := strings.TrimSpace(fields[0])
	cidr := strings.TrimSpace(fields[1])
	result.ASN = &asn
	result.CIDR = &cidr

	// Look up ASN org name
	asnQuery := fmt.Sprintf("AS%s.asn.cymru.com", asn)
	orgTxts, err := net.LookupTXT(asnQuery)
	if err == nil && len(orgTxts) > 0 {
		orgFields := strings.Split(orgTxts[0], "|")
		if len(orgFields) >= 5 {
			org := strings.TrimSpace(orgFields[4])
			result.Org = &org
		}
	}

	return result
}

func extractField(raw string, keys ...string) *string {
	for _, line := range strings.Split(raw, "\n") {
		trimmed := strings.TrimSpace(line)
		for _, key := range keys {
			if strings.HasPrefix(trimmed, key) {
				value := strings.TrimSpace(strings.TrimPrefix(trimmed, key))
				if value != "" {
					return &value
				}
			}
		}
	}
	return nil
}

func extractMultiple(raw string, keys ...string) []string {
	var results []string
	for _, line := range strings.Split(raw, "\n") {
		trimmed := strings.TrimSpace(line)
		for _, key := range keys {
			if strings.HasPrefix(trimmed, key) {
				value := strings.TrimSpace(strings.TrimPrefix(trimmed, key))
				if value != "" {
					results = append(results, strings.ToLower(value))
				}
			}
		}
	}
	return results
}
