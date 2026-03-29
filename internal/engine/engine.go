package engine

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"sort"
	"sync"
	"time"

	"reconx/internal/config"
	"reconx/internal/httpkit"
	"reconx/internal/models"
	"reconx/internal/workflow"
)

func stringOrDefault(s *string, def string) string {
	if s == nil {
		return def
	}
	return *s
}

// Engine orchestrates the recon pipeline.
type Engine struct {
	db       *sql.DB
	tools    map[PhaseID][]ToolRunner
	eventBus *EventBus
	config   *config.Config
}

// New creates a new engine instance.
func New(db *sql.DB, cfg *config.Config, eventBus *EventBus) *Engine {
	return &Engine{
		db:       db,
		tools:    make(map[PhaseID][]ToolRunner),
		eventBus: eventBus,
		config:   cfg,
	}
}

// EventBus returns the engine's event bus.
func (e *Engine) Bus() *EventBus {
	return e.eventBus
}

// RegisterTool adds a tool runner to the engine.
func (e *Engine) RegisterTool(t ToolRunner) {
	phase := t.Phase()
	e.tools[phase] = append(e.tools[phase], t)
}

// RunWorkflow executes a workflow for a workspace.
func (e *Engine) RunWorkflow(ctx context.Context, workspaceID string, wf *workflow.Workflow) error {
	phases := wf.EnabledPhases()
	phaseIDs := make([]PhaseID, 0, len(phases))
	enabledTools := make(map[PhaseID]map[string]bool)

	for _, p := range phases {
		pid := PhaseID(p.ID)
		phaseIDs = append(phaseIDs, pid)
		enabledTools[pid] = make(map[string]bool)
		for _, t := range p.EnabledTools() {
			enabledTools[pid][t.Name] = true
		}
	}

	return e.runPhases(ctx, workspaceID, phaseIDs, enabledTools)
}

// RunPhases executes specific phases for a workspace.
func (e *Engine) RunPhases(ctx context.Context, workspaceID string, phases []int) error {
	phaseIDs := make([]PhaseID, len(phases))
	for i, p := range phases {
		phaseIDs[i] = PhaseID(p)
	}
	return e.runPhases(ctx, workspaceID, phaseIDs, nil)
}

// RunTool executes a single tool for a workspace.
func (e *Engine) RunTool(ctx context.Context, workspaceID string, toolName string) error {
	for phaseID, tools := range e.tools {
		for _, t := range tools {
			if t.Name() == toolName {
				input, err := e.buildInput(ctx, workspaceID, phaseID)
				if err != nil {
					return err
				}
				return e.runSingleTool(ctx, workspaceID, phaseID, t, input)
			}
		}
	}
	return fmt.Errorf("tool %q not found", toolName)
}

// RunToolWithTargets executes a single tool scoped to specific targets.
// Used in interactive recon mode where users select items to operate on.
func (e *Engine) RunToolWithTargets(ctx context.Context, workspaceID string, toolName string, targets *TargetFilter) error {
	if targets.IsEmpty() {
		return e.RunTool(ctx, workspaceID, toolName)
	}
	for phaseID, tools := range e.tools {
		for _, t := range tools {
			if t.Name() == toolName {
				input, err := e.buildFilteredInput(ctx, workspaceID, phaseID, targets)
				if err != nil {
					return err
				}
				return e.runSingleTool(ctx, workspaceID, phaseID, t, input)
			}
		}
	}
	return fmt.Errorf("tool %q not found", toolName)
}

func (e *Engine) runPhases(ctx context.Context, workspaceID string, phases []PhaseID, enabledTools map[PhaseID]map[string]bool) error {
	// Sort phases in order
	sort.Slice(phases, func(i, j int) bool { return phases[i] < phases[j] })

	e.eventBus.Publish(Event{
		Type:        EventScanStarted,
		WorkspaceID: workspaceID,
		Data:        map[string]any{"phases": phases},
	})

	for _, phaseID := range phases {
		if err := ctx.Err(); err != nil {
			return err
		}

		tools := e.toolsForPhase(phaseID, enabledTools)
		if len(tools) == 0 {
			continue
		}

		e.eventBus.Publish(Event{
			Type:        EventPhaseStarted,
			WorkspaceID: workspaceID,
			Phase:       int(phaseID),
			Data:        map[string]any{"phase_name": PhaseName(phaseID), "tool_count": len(tools)},
		})

		input, err := e.buildInput(ctx, workspaceID, phaseID)
		if err != nil {
			return fmt.Errorf("building input for phase %d: %w", phaseID, err)
		}

		// Run tools concurrently within the phase
		if err := e.runToolsConcurrently(ctx, workspaceID, phaseID, tools, input); err != nil {
			return fmt.Errorf("phase %d: %w", phaseID, err)
		}

		// After subdomain enumeration: ensure the root domain is always a subdomain
		if phaseID == PhaseSubdomains {
			e.ensureRootDomain(ctx, workspaceID, input.Workspace.Domain)
		}

		e.eventBus.Publish(Event{
			Type:        EventPhaseCompleted,
			WorkspaceID: workspaceID,
			Phase:       int(phaseID),
		})
	}

	e.eventBus.Publish(Event{
		Type:        EventScanCompleted,
		WorkspaceID: workspaceID,
	})

	return nil
}

func (e *Engine) toolsForPhase(phaseID PhaseID, enabledTools map[PhaseID]map[string]bool) []ToolRunner {
	allTools := e.tools[phaseID]
	if enabledTools == nil {
		// No filter — return all tools that are enabled in config
		var result []ToolRunner
		for _, t := range allTools {
			if e.config.IsToolEnabled(t.Name()) {
				result = append(result, t)
			}
		}
		return result
	}

	allowed, ok := enabledTools[phaseID]
	if !ok {
		return nil
	}

	var result []ToolRunner
	for _, t := range allTools {
		if allowed[t.Name()] && e.config.IsToolEnabled(t.Name()) {
			result = append(result, t)
		}
	}
	return result
}

func (e *Engine) runToolsConcurrently(ctx context.Context, workspaceID string, phaseID PhaseID, tools []ToolRunner, input *PhaseInput) error {
	sem := make(chan struct{}, e.config.General.MaxConcurrentTools)
	var wg sync.WaitGroup
	var mu sync.Mutex
	var errs []error

	for _, t := range tools {
		wg.Add(1)
		go func(tool ToolRunner) {
			defer wg.Done()

			sem <- struct{}{}
			defer func() { <-sem }()

			if err := e.runSingleTool(ctx, workspaceID, phaseID, tool, input); err != nil {
				mu.Lock()
				errs = append(errs, fmt.Errorf("%s: %w", tool.Name(), err))
				mu.Unlock()
			}
		}(t)
	}

	wg.Wait()

	if len(errs) > 0 {
		// Log errors but don't fail the whole phase — some tools may have succeeded
		for _, err := range errs {
			e.eventBus.Publish(Event{
				Type:        EventScanFailed,
				WorkspaceID: workspaceID,
				Phase:       int(phaseID),
				Data:        map[string]string{"error": err.Error()},
			})
		}
	}

	return nil
}

func (e *Engine) runSingleTool(ctx context.Context, workspaceID string, phaseID PhaseID, tool ToolRunner, input *PhaseInput) error {
	// Enforce per-tool timeout
	tc := e.config.GetToolConfig(tool.Name())
	toolTimeout := 30 * time.Minute
	if tc.Timeout != "" {
		if d, err := time.ParseDuration(tc.Timeout); err == nil {
			toolTimeout = d
		}
	}
	toolCtx, toolCancel := context.WithTimeout(ctx, toolTimeout)
	defer toolCancel()

	// Create scan job
	jobID := models.NewID()
	now := models.Now()
	_, err := e.db.ExecContext(toolCtx, `
		INSERT INTO scan_jobs (id, workspace_id, phase, tool_name, status, started_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, jobID, workspaceID, int(phaseID), tool.Name(), models.ScanStatusRunning, now, now)
	if err != nil {
		return fmt.Errorf("creating scan job: %w", err)
	}

	input.ScanJobID = jobID
	sink := NewDBSink(e.db, e.eventBus, workspaceID, jobID, int(phaseID), tool.Name())

	e.eventBus.Publish(Event{
		Type:        EventScanStarted,
		WorkspaceID: workspaceID,
		ScanJobID:   jobID,
		Phase:       int(phaseID),
		ToolName:    tool.Name(),
	})

	// Execute the tool with enforced timeout
	runErr := tool.Run(toolCtx, input, sink)

	// Update scan job status
	finishedAt := models.Now()
	status := models.ScanStatusCompleted
	var errMsg *string

	if toolCtx.Err() == context.DeadlineExceeded {
		status = models.ScanStatusFailed
		msg := fmt.Sprintf("killed: exceeded timeout (%s)", toolTimeout)
		errMsg = &msg
	} else if runErr != nil {
		status = models.ScanStatusFailed
		msg := runErr.Error()
		errMsg = &msg
	} else if ctx.Err() != nil {
		status = models.ScanStatusCancelled
	}

	// Use background context for DB update (toolCtx may be cancelled)
	e.db.Exec(`
		UPDATE scan_jobs SET status = ?, finished_at = ?, error_message = ? WHERE id = ?
	`, status, finishedAt, errMsg, jobID)

	// Result summary logging
	var resultCount int
	e.db.QueryRow("SELECT result_count FROM scan_jobs WHERE id = ?", jobID).Scan(&resultCount)

	if status == models.ScanStatusFailed {
		sink.LogLine(context.Background(), "stderr", fmt.Sprintf("FAILED: %s", stringOrDefault(errMsg, "unknown error")))
		e.eventBus.Publish(Event{
			Type:        EventScanFailed,
			WorkspaceID: workspaceID,
			ScanJobID:   jobID,
			Phase:       int(phaseID),
			ToolName:    tool.Name(),
			Data:        map[string]any{"error": stringOrDefault(errMsg, "unknown"), "result_count": resultCount},
		})
	} else {
		summary := fmt.Sprintf("completed: %d results found", resultCount)
		if resultCount == 0 {
			summary = "completed successfully — 0 findings"
		}
		sink.LogLine(context.Background(), "stdout", summary)
		e.eventBus.Publish(Event{
			Type:        EventScanCompleted,
			WorkspaceID: workspaceID,
			ScanJobID:   jobID,
			Phase:       int(phaseID),
			ToolName:    tool.Name(),
			Data:        map[string]any{"result_count": resultCount},
		})
	}

	return runErr
}

// buildInput gathers data from previous phases as input for the current phase.
func (e *Engine) buildInput(ctx context.Context, workspaceID string, phaseID PhaseID) (*PhaseInput, error) {
	input := &PhaseInput{
		Config: e.config,
	}

	// Load workspace
	var ws models.Workspace
	err := e.db.QueryRowContext(ctx, "SELECT id, name, domain, description, config_json, created_at, updated_at FROM workspaces WHERE id = ?", workspaceID).
		Scan(&ws.ID, &ws.Name, &ws.Domain, &ws.Description, &ws.ConfigJSON, &ws.CreatedAt, &ws.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("loading workspace: %w", err)
	}
	input.Workspace = &ws
	input.ProxyURL = e.config.Proxy.URL

	// Load subdomains if needed (phases 3+)
	if phaseID >= PhasePorts {
		rows, err := e.db.QueryContext(ctx, "SELECT id, workspace_id, hostname, ip_addresses, is_alive, source, first_seen, last_seen, scan_job_id FROM subdomains WHERE workspace_id = ?", workspaceID)
		if err != nil {
			return nil, fmt.Errorf("loading subdomains: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			var s models.Subdomain
			if err := rows.Scan(&s.ID, &s.WorkspaceID, &s.Hostname, &s.IPAddresses, &s.IsAlive, &s.Source, &s.FirstSeen, &s.LastSeen, &s.ScanJobID); err != nil {
				return nil, err
			}
			input.Subdomains = append(input.Subdomains, s)
		}
	}

	// Load ports if needed (phases 4+)
	if phaseID >= PhaseFingerprint {
		rows, err := e.db.QueryContext(ctx, "SELECT id, workspace_id, subdomain_id, ip_address, port, protocol, state, service, version, banner, scan_job_id, created_at FROM ports WHERE workspace_id = ? AND state = 'open'", workspaceID)
		if err != nil {
			return nil, fmt.Errorf("loading ports: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			var p models.Port
			if err := rows.Scan(&p.ID, &p.WorkspaceID, &p.SubdomainID, &p.IPAddress, &p.Port, &p.Protocol, &p.State, &p.Service, &p.Version, &p.Banner, &p.ScanJobID, &p.CreatedAt); err != nil {
				return nil, err
			}
			input.Ports = append(input.Ports, p)
		}
	}

	// Load technologies and classifications if needed (phases 5+)
	if phaseID >= PhaseContent {
		rows, err := e.db.QueryContext(ctx, "SELECT id, workspace_id, subdomain_id, url, name, version, category, confidence, scan_job_id, created_at FROM technologies WHERE workspace_id = ?", workspaceID)
		if err != nil {
			return nil, fmt.Errorf("loading technologies: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			var t models.Technology
			if err := rows.Scan(&t.ID, &t.WorkspaceID, &t.SubdomainID, &t.URL, &t.Name, &t.Version, &t.Category, &t.Confidence, &t.ScanJobID, &t.CreatedAt); err != nil {
				return nil, err
			}
			input.Technologies = append(input.Technologies, t)
		}

		cRows, err := e.db.QueryContext(ctx, "SELECT id, workspace_id, subdomain_id, url, site_type, infra_type, waf_detected, cdn_detected, ssl_grade, ssl_details, evidence, scan_job_id, created_at FROM site_classifications WHERE workspace_id = ?", workspaceID)
		if err != nil {
			return nil, fmt.Errorf("loading classifications: %w", err)
		}
		defer cRows.Close()
		for cRows.Next() {
			var c models.SiteClassification
			if err := cRows.Scan(&c.ID, &c.WorkspaceID, &c.SubdomainID, &c.URL, &c.SiteType, &c.InfraType, &c.WAFDetected, &c.CDNDetected, &c.SSLGrade, &c.SSLDetails, &c.Evidence, &c.ScanJobID, &c.CreatedAt); err != nil {
				return nil, err
			}
			input.Classifications = append(input.Classifications, c)
		}

		dnsRows, err := e.db.QueryContext(ctx,
			"SELECT id, workspace_id, host, record_type, value, ttl, priority, scan_job_id, created_at FROM dns_records WHERE workspace_id = ?",
			workspaceID)
		if err == nil {
			defer dnsRows.Close()
			for dnsRows.Next() {
				var record models.DNSRecord
				if err := dnsRows.Scan(&record.ID, &record.WorkspaceID, &record.Host, &record.RecordType, &record.Value, &record.TTL, &record.Priority, &record.ScanJobID, &record.CreatedAt); err != nil {
					continue
				}
				input.DNSRecords = append(input.DNSRecords, record)
			}
		}

		whoisRows, err := e.db.QueryContext(ctx,
			"SELECT id, workspace_id, domain, registrar, org, country, creation_date, expiry_date, name_servers, raw, asn, asn_org, asn_cidr, scan_job_id, created_at FROM whois_records WHERE workspace_id = ?",
			workspaceID)
		if err == nil {
			defer whoisRows.Close()
			for whoisRows.Next() {
				var record models.WhoisRecord
				if err := whoisRows.Scan(&record.ID, &record.WorkspaceID, &record.Domain, &record.Registrar, &record.Org, &record.Country, &record.CreationDate, &record.ExpiryDate, &record.NameServers, &record.Raw, &record.ASN, &record.ASNOrg, &record.ASNCIDR, &record.ScanJobID, &record.CreatedAt); err != nil {
					continue
				}
				input.WhoisRecords = append(input.WhoisRecords, record)
			}
		}
	}

	// Load discovered URLs and historical URLs for JS analysis (phases 5+)
	if phaseID >= PhaseContent {
		dRows, err := e.db.QueryContext(ctx,
			"SELECT id, workspace_id, subdomain_id, url, status_code, content_type, content_length, redirect_location, source, scan_job_id, created_at FROM discovered_urls WHERE workspace_id = ?",
			workspaceID)
		if err == nil {
			defer dRows.Close()
			for dRows.Next() {
				var u models.DiscoveredURL
				if err := dRows.Scan(&u.ID, &u.WorkspaceID, &u.SubdomainID, &u.URL, &u.StatusCode, &u.ContentType, &u.ContentLength, &u.RedirectLocation, &u.Source, &u.ScanJobID, &u.CreatedAt); err != nil {
					continue
				}
				input.DiscoveredURLs = append(input.DiscoveredURLs, u)
			}
		}

		hRows, err := e.db.QueryContext(ctx,
			"SELECT id, workspace_id, url, source, scan_job_id, created_at FROM historical_urls WHERE workspace_id = ?",
			workspaceID)
		if err == nil {
			defer hRows.Close()
			for hRows.Next() {
				var u models.HistoricalURL
				if err := hRows.Scan(&u.ID, &u.WorkspaceID, &u.URL, &u.Source, &u.ScanJobID, &u.CreatedAt); err != nil {
					continue
				}
				input.HistoricalURLs = append(input.HistoricalURLs, u)
			}
		}
	}

	// Load auth credentials for authenticated crawling (phases 5+)
	if phaseID >= PhaseContent {
		authRows, err := e.db.QueryContext(ctx,
			`SELECT id, workspace_id, name, auth_type, username, password, login_url, login_body,
				token, header_name, header_value, is_active, created_at, updated_at
			FROM auth_credentials WHERE workspace_id = ? AND is_active = 1`,
			workspaceID)
		if err == nil {
			defer authRows.Close()
			for authRows.Next() {
				var cred models.AuthCredential
				if err := authRows.Scan(&cred.ID, &cred.WorkspaceID, &cred.Name, &cred.AuthType,
					&cred.Username, &cred.Password, &cred.LoginURL, &cred.LoginBody,
					&cred.Token, &cred.HeaderName, &cred.HeaderValue,
					&cred.IsActive, &cred.CreatedAt, &cred.UpdatedAt); err != nil {
					continue
				}
				sess := httpkit.NewAuthSession(&cred)
				if err := sess.Login(ctx, httpkit.NewClient(e.config)); err != nil {
					e.eventBus.Publish(Event{
						Type:        EventScanFailed,
						WorkspaceID: workspaceID,
						Data:        map[string]string{"error": fmt.Sprintf("auth login failed for %s: %v", cred.Name, err)},
					})
					continue
				}
				input.AuthSessions = append(input.AuthSessions, sess)
			}
		}
	}

	return input, nil
}

// buildFilteredInput gathers data like buildInput but scopes to specific targets.
func (e *Engine) buildFilteredInput(ctx context.Context, workspaceID string, phaseID PhaseID, targets *TargetFilter) (*PhaseInput, error) {
	// Start with full input, then filter
	input, err := e.buildInput(ctx, workspaceID, phaseID)
	if err != nil {
		return nil, err
	}

	// Filter subdomains by ID or hostname
	if len(targets.SubdomainIDs) > 0 || len(targets.Hostnames) > 0 {
		idSet := make(map[string]bool, len(targets.SubdomainIDs))
		for _, id := range targets.SubdomainIDs {
			idSet[id] = true
		}
		hostSet := make(map[string]bool, len(targets.Hostnames))
		for _, h := range targets.Hostnames {
			hostSet[h] = true
		}

		filtered := make([]models.Subdomain, 0, len(targets.SubdomainIDs)+len(targets.Hostnames))
		for _, s := range input.Subdomains {
			if idSet[s.ID] || hostSet[s.Hostname] {
				filtered = append(filtered, s)
			}
		}
		input.Subdomains = filtered

		// Ensure host-targeted chaining works across modules even when hostnames
		// are not already present as stored subdomain rows.
		existingHosts := make(map[string]bool, len(input.Subdomains))
		for _, s := range input.Subdomains {
			existingHosts[s.Hostname] = true
		}
		for _, host := range targets.Hostnames {
			if host == "" || existingHosts[host] {
				continue
			}
			existingHosts[host] = true
			input.Subdomains = append(input.Subdomains, models.Subdomain{
				Hostname: host,
				IsAlive:  true,
			})
		}
	}

	// Filter ports by ID
	if len(targets.PortIDs) > 0 {
		idSet := make(map[string]bool, len(targets.PortIDs))
		for _, id := range targets.PortIDs {
			idSet[id] = true
		}
		filtered := make([]models.Port, 0, len(targets.PortIDs))
		for _, p := range input.Ports {
			if idSet[p.ID] {
				filtered = append(filtered, p)
			}
		}
		input.Ports = filtered
	}

	// Filter URLs by ID
	if len(targets.URLIDs) > 0 {
		idSet := make(map[string]bool, len(targets.URLIDs))
		for _, id := range targets.URLIDs {
			idSet[id] = true
		}
		filtered := make([]models.DiscoveredURL, 0, len(targets.URLIDs))
		for _, u := range input.DiscoveredURLs {
			if idSet[u.ID] {
				filtered = append(filtered, u)
			}
		}
		input.DiscoveredURLs = filtered

		// Cross-populate: extract unique hostnames from selected URLs and add
		// synthetic subdomains so tools that work with hostnames (waf_detect,
		// classify, httpx, ssl_analyze, nmap, etc.) can target them.
		existingHosts := make(map[string]bool, len(input.Subdomains))
		for _, s := range input.Subdomains {
			existingHosts[s.Hostname] = true
		}
		for _, u := range filtered {
			if parsed, err := url.Parse(u.URL); err == nil && parsed.Hostname() != "" {
				host := parsed.Hostname()
				if !existingHosts[host] {
					existingHosts[host] = true
					input.Subdomains = append(input.Subdomains, models.Subdomain{
						Hostname: host,
						IsAlive:  true, // we have a URL for it, so it responded
					})
				}
			}
		}
	}

	return input, nil
}

// ensureRootDomain adds the workspace's root domain as a subdomain if no subdomains were found.
// This guarantees phases 3+ always have at least one target to scan.
func (e *Engine) ensureRootDomain(ctx context.Context, workspaceID, domain string) {
	var count int
	e.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM subdomains WHERE workspace_id = ?", workspaceID).Scan(&count)
	if count > 0 {
		return
	}

	// Resolve IPs
	var ipStr *string
	addrs, err := net.LookupHost(domain)
	if err == nil && len(addrs) > 0 {
		ipJSON, _ := json.Marshal(addrs)
		s := string(ipJSON)
		ipStr = &s
	}

	id := models.NewID()
	now := models.Now()
	e.db.ExecContext(ctx, `
		INSERT INTO subdomains (id, workspace_id, hostname, ip_addresses, is_alive, source, first_seen, last_seen)
		VALUES (?, ?, ?, ?, 1, 'root_domain', ?, ?)
		ON CONFLICT(workspace_id, hostname) DO NOTHING
	`, id, workspaceID, domain, ipStr, now, now)

	e.eventBus.Publish(Event{
		Type:        EventNewSubdomain,
		WorkspaceID: workspaceID,
		Data:        map[string]string{"hostname": domain, "source": "root_domain"},
	})
}

// CheckTools verifies which registered tools are available on the system.
func (e *Engine) CheckTools() map[string]error {
	results := make(map[string]error)
	for _, tools := range e.tools {
		for _, t := range tools {
			results[t.Name()] = t.Check()
		}
	}
	return results
}

// RegisteredTools returns a list of all registered tool names grouped by phase.
func (e *Engine) RegisteredTools() map[int][]string {
	result := make(map[int][]string)
	for phaseID, tools := range e.tools {
		for _, t := range tools {
			result[int(phaseID)] = append(result[int(phaseID)], t.Name())
		}
	}
	return result
}
