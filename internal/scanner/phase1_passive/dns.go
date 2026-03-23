package phase1

import (
	"context"
	"fmt"
	"net"
	"strings"

	"reconx/internal/engine"
	"reconx/internal/models"
	"reconx/internal/tools"
)

// DNSRunner performs deep DNS enumeration on the target domain.
type DNSRunner struct{}

func (d *DNSRunner) Name() string         { return "dns" }
func (d *DNSRunner) Phase() engine.PhaseID { return engine.PhasePassive }
func (d *DNSRunner) Check() error          { return tools.CheckBinary("dig") }

func (d *DNSRunner) Run(ctx context.Context, input *engine.PhaseInput, sink engine.ResultSink) error {
	domain := input.Workspace.Domain

	recordTypes := []string{"A", "AAAA", "MX", "TXT", "NS", "SOA", "CNAME", "SRV"}

	for _, rtype := range recordTypes {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := d.queryRecordType(ctx, domain, rtype, sink); err != nil {
			sink.LogLine(ctx, "stderr", fmt.Sprintf("DNS %s query failed: %v", rtype, err))
		}
	}

	// Attempt zone transfer (AXFR)
	d.attemptAXFR(ctx, domain, sink)

	return nil
}

func (d *DNSRunner) queryRecordType(ctx context.Context, domain, rtype string, sink engine.ResultSink) error {
	var records []dnsEntry

	switch rtype {
	case "A":
		ips, err := net.LookupHost(domain)
		if err != nil {
			return err
		}
		for _, ip := range ips {
			if net.ParseIP(ip).To4() != nil {
				records = append(records, dnsEntry{Value: ip})
			}
		}
	case "AAAA":
		ips, err := net.LookupHost(domain)
		if err != nil {
			return err
		}
		for _, ip := range ips {
			if net.ParseIP(ip).To4() == nil {
				records = append(records, dnsEntry{Value: ip})
			}
		}
	case "MX":
		mxs, err := net.LookupMX(domain)
		if err != nil {
			return err
		}
		for _, mx := range mxs {
			prio := int(mx.Pref)
			records = append(records, dnsEntry{Value: strings.TrimSuffix(mx.Host, "."), Priority: &prio})
		}
	case "TXT":
		txts, err := net.LookupTXT(domain)
		if err != nil {
			return err
		}
		for _, txt := range txts {
			records = append(records, dnsEntry{Value: txt})
		}
	case "NS":
		nss, err := net.LookupNS(domain)
		if err != nil {
			return err
		}
		for _, ns := range nss {
			records = append(records, dnsEntry{Value: strings.TrimSuffix(ns.Host, ".")})
		}
	case "SOA", "CNAME", "SRV":
		// Use dig for these record types since net package doesn't support them directly
		return d.queryWithDig(ctx, domain, rtype, sink)
	}

	for _, rec := range records {
		r := &models.DNSRecord{
			Host:       domain,
			RecordType: rtype,
			Value:      rec.Value,
			Priority:   rec.Priority,
		}
		sink.LogLine(ctx, "stdout", fmt.Sprintf("%s %s %s", domain, rtype, rec.Value))
		if err := sink.AddDNSRecord(ctx, r); err != nil {
			return err
		}
	}

	return nil
}

func (d *DNSRunner) queryWithDig(ctx context.Context, domain, rtype string, sink engine.ResultSink) error {
	result, err := tools.RunToolWithProxy(ctx, "dig", []string{"+short", "+noall", "+answer", domain, rtype}, "", func(stream, line string) {
		sink.LogLine(ctx, stream, line)
		if stream == "stdout" && strings.TrimSpace(line) != "" {
			r := &models.DNSRecord{
				Host:       domain,
				RecordType: rtype,
				Value:      strings.TrimSpace(line),
			}
			sink.AddDNSRecord(ctx, r)
		}
	})
	if err != nil {
		return err
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("dig %s %s failed with exit code %d", domain, rtype, result.ExitCode)
	}
	return nil
}

func (d *DNSRunner) attemptAXFR(ctx context.Context, domain string, sink engine.ResultSink) {
	// Get nameservers first
	nss, err := net.LookupNS(domain)
	if err != nil || len(nss) == 0 {
		return
	}

	for _, ns := range nss {
		nsHost := strings.TrimSuffix(ns.Host, ".")
		sink.LogLine(ctx, "stdout", fmt.Sprintf("Attempting AXFR via %s...", nsHost))

		result, err := tools.RunToolWithProxy(ctx, "dig", []string{"@" + nsHost, domain, "AXFR", "+short"}, "", func(stream, line string) {
			sink.LogLine(ctx, stream, line)
			if stream == "stdout" && strings.TrimSpace(line) != "" && !strings.HasPrefix(line, ";") {
				r := &models.DNSRecord{
					Host:       domain,
					RecordType: "AXFR",
					Value:      strings.TrimSpace(line),
				}
				sink.AddDNSRecord(ctx, r)
			}
		})
		if err != nil {
			sink.LogLine(ctx, "stderr", fmt.Sprintf("AXFR via %s failed: %v", nsHost, err))
			continue
		}
		if result.ExitCode == 0 {
			sink.LogLine(ctx, "stdout", fmt.Sprintf("AXFR via %s completed", nsHost))
			return // Success, no need to try other nameservers
		}
	}
}

type dnsEntry struct {
	Value    string
	Priority *int
}
