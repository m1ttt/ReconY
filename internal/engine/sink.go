package engine

import (
	"context"
	"database/sql"
	"fmt"

	"reconx/internal/models"
)

// DBSink implements ResultSink by writing to SQLite and publishing events.
type DBSink struct {
	db          *sql.DB
	eventBus    *EventBus
	workspaceID string
	scanJobID   string
	phase       int
	toolName    string
	resultCount int
}

// NewDBSink creates a new database-backed result sink.
func NewDBSink(db *sql.DB, eventBus *EventBus, workspaceID, scanJobID string, phase int, toolName string) *DBSink {
	return &DBSink{
		db:          db,
		eventBus:    eventBus,
		workspaceID: workspaceID,
		scanJobID:   scanJobID,
		phase:       phase,
		toolName:    toolName,
	}
}

func (s *DBSink) emit(eventType EventType, data any) {
	s.eventBus.Publish(Event{
		Type:        eventType,
		WorkspaceID: s.workspaceID,
		ScanJobID:   s.scanJobID,
		Phase:       s.phase,
		ToolName:    s.toolName,
		Data:        data,
	})
}

func (s *DBSink) AddSubdomain(ctx context.Context, sub *models.Subdomain) error {
	if sub.ID == "" {
		sub.ID = models.NewID()
	}
	sub.WorkspaceID = s.workspaceID
	sub.ScanJobID = &s.scanJobID
	now := models.Now()
	if sub.FirstSeen == "" {
		sub.FirstSeen = now
	}
	sub.LastSeen = now

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO subdomains (id, workspace_id, hostname, ip_addresses, is_alive, source, first_seen, last_seen, scan_job_id)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(workspace_id, hostname) DO UPDATE SET
			last_seen = excluded.last_seen,
			ip_addresses = COALESCE(excluded.ip_addresses, ip_addresses),
			is_alive = CASE WHEN excluded.is_alive = 1 THEN 1 ELSE is_alive END,
			scan_job_id = excluded.scan_job_id
	`, sub.ID, sub.WorkspaceID, sub.Hostname, sub.IPAddresses, sub.IsAlive, sub.Source, sub.FirstSeen, sub.LastSeen, sub.ScanJobID)
	if err != nil {
		return fmt.Errorf("inserting subdomain: %w", err)
	}

	s.emit(EventNewSubdomain, sub)
	s.incrementResultCount(ctx)
	return nil
}

func (s *DBSink) AddPort(ctx context.Context, p *models.Port) error {
	if p.ID == "" {
		p.ID = models.NewID()
	}
	p.WorkspaceID = s.workspaceID
	p.ScanJobID = &s.scanJobID
	if p.CreatedAt == "" {
		p.CreatedAt = models.Now()
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO ports (id, workspace_id, subdomain_id, ip_address, port, protocol, state, service, version, banner, scan_job_id, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(workspace_id, ip_address, port, protocol) DO UPDATE SET
			state = excluded.state,
			service = COALESCE(excluded.service, service),
			version = COALESCE(excluded.version, version),
			banner = COALESCE(excluded.banner, banner),
			scan_job_id = excluded.scan_job_id
	`, p.ID, p.WorkspaceID, p.SubdomainID, p.IPAddress, p.Port, p.Protocol, p.State, p.Service, p.Version, p.Banner, p.ScanJobID, p.CreatedAt)
	if err != nil {
		return fmt.Errorf("inserting port: %w", err)
	}

	s.emit(EventNewPort, p)
	s.incrementResultCount(ctx)
	return nil
}

func (s *DBSink) AddTechnology(ctx context.Context, t *models.Technology) error {
	if t.ID == "" {
		t.ID = models.NewID()
	}
	t.WorkspaceID = s.workspaceID
	t.ScanJobID = &s.scanJobID
	if t.CreatedAt == "" {
		t.CreatedAt = models.Now()
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO technologies (id, workspace_id, subdomain_id, url, name, version, category, confidence, scan_job_id, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(workspace_id, url, name) DO UPDATE SET
			version = COALESCE(excluded.version, version),
			confidence = MAX(excluded.confidence, confidence),
			scan_job_id = excluded.scan_job_id
	`, t.ID, t.WorkspaceID, t.SubdomainID, t.URL, t.Name, t.Version, t.Category, t.Confidence, t.ScanJobID, t.CreatedAt)
	if err != nil {
		return fmt.Errorf("inserting technology: %w", err)
	}

	s.emit(EventNewTech, t)
	s.incrementResultCount(ctx)
	return nil
}

func (s *DBSink) AddVulnerability(ctx context.Context, v *models.Vulnerability) error {
	if v.ID == "" {
		v.ID = models.NewID()
	}
	v.WorkspaceID = s.workspaceID
	v.ScanJobID = &s.scanJobID
	if v.CreatedAt == "" {
		v.CreatedAt = models.Now()
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO vulnerabilities (id, workspace_id, subdomain_id, template_id, name, severity, url, matched_at, description, reference, curl_command, extracted, scan_job_id, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(workspace_id, template_id, url) DO UPDATE SET
			scan_job_id = excluded.scan_job_id
	`, v.ID, v.WorkspaceID, v.SubdomainID, v.TemplateID, v.Name, v.Severity, v.URL, v.MatchedAt, v.Description, v.Reference, v.CurlCommand, v.Extracted, v.ScanJobID, v.CreatedAt)
	if err != nil {
		return fmt.Errorf("inserting vulnerability: %w", err)
	}

	s.emit(EventNewVuln, v)
	s.incrementResultCount(ctx)
	return nil
}

func (s *DBSink) AddDNSRecord(ctx context.Context, r *models.DNSRecord) error {
	if r.ID == "" {
		r.ID = models.NewID()
	}
	r.WorkspaceID = s.workspaceID
	r.ScanJobID = &s.scanJobID
	if r.CreatedAt == "" {
		r.CreatedAt = models.Now()
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO dns_records (id, workspace_id, host, record_type, value, ttl, priority, scan_job_id, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, r.ID, r.WorkspaceID, r.Host, r.RecordType, r.Value, r.TTL, r.Priority, r.ScanJobID, r.CreatedAt)
	if err != nil {
		return fmt.Errorf("inserting dns record: %w", err)
	}

	s.incrementResultCount(ctx)
	return nil
}

func (s *DBSink) AddWhoisRecord(ctx context.Context, w *models.WhoisRecord) error {
	if w.ID == "" {
		w.ID = models.NewID()
	}
	w.WorkspaceID = s.workspaceID
	w.ScanJobID = &s.scanJobID
	if w.CreatedAt == "" {
		w.CreatedAt = models.Now()
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO whois_records (id, workspace_id, domain, registrar, org, country, creation_date, expiry_date, name_servers, raw, asn, asn_org, asn_cidr, scan_job_id, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, w.ID, w.WorkspaceID, w.Domain, w.Registrar, w.Org, w.Country, w.CreationDate, w.ExpiryDate, w.NameServers, w.Raw, w.ASN, w.ASNOrg, w.ASNCIDR, w.ScanJobID, w.CreatedAt)
	if err != nil {
		return fmt.Errorf("inserting whois record: %w", err)
	}

	s.incrementResultCount(ctx)
	return nil
}

func (s *DBSink) AddHistoricalURL(ctx context.Context, u *models.HistoricalURL) error {
	if u.ID == "" {
		u.ID = models.NewID()
	}
	u.WorkspaceID = s.workspaceID
	u.ScanJobID = &s.scanJobID
	if u.CreatedAt == "" {
		u.CreatedAt = models.Now()
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO historical_urls (id, workspace_id, url, source, scan_job_id, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(workspace_id, url) DO UPDATE SET
			scan_job_id = excluded.scan_job_id
	`, u.ID, u.WorkspaceID, u.URL, u.Source, u.ScanJobID, u.CreatedAt)
	if err != nil {
		return fmt.Errorf("inserting historical url: %w", err)
	}

	s.emit(EventNewURL, u)
	s.incrementResultCount(ctx)
	return nil
}

func (s *DBSink) AddDiscoveredURL(ctx context.Context, u *models.DiscoveredURL) error {
	if u.ID == "" {
		u.ID = models.NewID()
	}
	u.WorkspaceID = s.workspaceID
	u.ScanJobID = &s.scanJobID
	if u.CreatedAt == "" {
		u.CreatedAt = models.Now()
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO discovered_urls (id, workspace_id, subdomain_id, url, status_code, content_type, content_length, redirect_location, source, scan_job_id, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(workspace_id, url) DO UPDATE SET
			status_code = COALESCE(excluded.status_code, status_code),
			content_type = COALESCE(excluded.content_type, content_type),
			redirect_location = COALESCE(excluded.redirect_location, redirect_location),
			scan_job_id = excluded.scan_job_id
	`, u.ID, u.WorkspaceID, u.SubdomainID, u.URL, u.StatusCode, u.ContentType, u.ContentLength, u.RedirectLocation, u.Source, u.ScanJobID, u.CreatedAt)
	if err != nil {
		return fmt.Errorf("inserting discovered url: %w", err)
	}

	s.emit(EventNewURL, u)
	s.incrementResultCount(ctx)
	return nil
}

func (s *DBSink) AddParameter(ctx context.Context, p *models.Parameter) error {
	if p.ID == "" {
		p.ID = models.NewID()
	}
	p.WorkspaceID = s.workspaceID
	p.ScanJobID = &s.scanJobID
	if p.CreatedAt == "" {
		p.CreatedAt = models.Now()
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO parameters (id, workspace_id, url, name, param_type, source, scan_job_id, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, p.ID, p.WorkspaceID, p.URL, p.Name, p.ParamType, p.Source, p.ScanJobID, p.CreatedAt)
	if err != nil {
		return fmt.Errorf("inserting parameter: %w", err)
	}

	s.incrementResultCount(ctx)
	return nil
}

func (s *DBSink) AddScreenshot(ctx context.Context, sc *models.Screenshot) error {
	if sc.ID == "" {
		sc.ID = models.NewID()
	}
	sc.WorkspaceID = s.workspaceID
	sc.ScanJobID = &s.scanJobID
	if sc.CreatedAt == "" {
		sc.CreatedAt = models.Now()
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO screenshots (id, workspace_id, subdomain_id, url, file_path, status_code, title, scan_job_id, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, sc.ID, sc.WorkspaceID, sc.SubdomainID, sc.URL, sc.FilePath, sc.StatusCode, sc.Title, sc.ScanJobID, sc.CreatedAt)
	if err != nil {
		return fmt.Errorf("inserting screenshot: %w", err)
	}

	s.emit(EventNewScreenshot, sc)
	s.incrementResultCount(ctx)
	return nil
}

func (s *DBSink) AddCloudAsset(ctx context.Context, c *models.CloudAsset) error {
	if c.ID == "" {
		c.ID = models.NewID()
	}
	c.WorkspaceID = s.workspaceID
	c.ScanJobID = &s.scanJobID
	if c.CreatedAt == "" {
		c.CreatedAt = models.Now()
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO cloud_assets (id, workspace_id, provider, asset_type, name, url, is_public, permissions, scan_job_id, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, c.ID, c.WorkspaceID, c.Provider, c.AssetType, c.Name, c.URL, c.IsPublic, c.Permissions, c.ScanJobID, c.CreatedAt)
	if err != nil {
		return fmt.Errorf("inserting cloud asset: %w", err)
	}

	s.emit(EventNewCloudAsset, c)
	s.incrementResultCount(ctx)
	return nil
}

func (s *DBSink) AddSecret(ctx context.Context, sec *models.Secret) error {
	if sec.ID == "" {
		sec.ID = models.NewID()
	}
	sec.WorkspaceID = s.workspaceID
	sec.ScanJobID = &s.scanJobID
	if sec.CreatedAt == "" {
		sec.CreatedAt = models.Now()
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO secrets (id, workspace_id, source_url, secret_type, value, context, source, severity, scan_job_id, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, sec.ID, sec.WorkspaceID, sec.SourceURL, sec.SecretType, sec.Value, sec.Context, sec.Source, sec.Severity, sec.ScanJobID, sec.CreatedAt)
	if err != nil {
		return fmt.Errorf("inserting secret: %w", err)
	}

	s.emit(EventNewSecret, sec)
	s.incrementResultCount(ctx)
	return nil
}

func (s *DBSink) AddSiteClassification(ctx context.Context, c *models.SiteClassification) error {
	if c.ID == "" {
		c.ID = models.NewID()
	}
	c.WorkspaceID = s.workspaceID
	c.ScanJobID = &s.scanJobID
	if c.CreatedAt == "" {
		c.CreatedAt = models.Now()
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO site_classifications (id, workspace_id, subdomain_id, url, site_type, infra_type, waf_detected, cdn_detected, ssl_grade, ssl_details, evidence, scan_job_id, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(workspace_id, url) DO UPDATE SET
			site_type = excluded.site_type,
			infra_type = COALESCE(excluded.infra_type, infra_type),
			waf_detected = COALESCE(excluded.waf_detected, waf_detected),
			cdn_detected = COALESCE(excluded.cdn_detected, cdn_detected),
			ssl_grade = COALESCE(excluded.ssl_grade, ssl_grade),
			ssl_details = COALESCE(excluded.ssl_details, ssl_details),
			evidence = COALESCE(excluded.evidence, evidence),
			scan_job_id = excluded.scan_job_id
	`, c.ID, c.WorkspaceID, c.SubdomainID, c.URL, c.SiteType, c.InfraType, c.WAFDetected, c.CDNDetected, c.SSLGrade, c.SSLDetails, c.Evidence, c.ScanJobID, c.CreatedAt)
	if err != nil {
		return fmt.Errorf("inserting site classification: %w", err)
	}

	s.incrementResultCount(ctx)
	return nil
}

func (s *DBSink) LogLine(ctx context.Context, stream string, line string) {
	logCtx := ctx
	if logCtx == nil || logCtx.Err() != nil {
		logCtx = context.Background()
	}

	s.db.ExecContext(logCtx, `
		INSERT INTO tool_logs (scan_job_id, stream, line) VALUES (?, ?, ?)
	`, s.scanJobID, stream, line)

	s.eventBus.Publish(Event{
		Type:        EventScanLogLine,
		WorkspaceID: s.workspaceID,
		ScanJobID:   s.scanJobID,
		Phase:       s.phase,
		ToolName:    s.toolName,
		Data:        map[string]string{"stream": stream, "line": line},
	})
}

func (s *DBSink) incrementResultCount(ctx context.Context) {
	s.resultCount++
	s.db.ExecContext(ctx, `
		UPDATE scan_jobs SET result_count = result_count + 1 WHERE id = ?
	`, s.scanJobID)

	// Emit progress every 10 results (or first 5 for quick feedback)
	if s.resultCount <= 5 || s.resultCount%10 == 0 {
		s.eventBus.Publish(Event{
			Type:        EventScanProgress,
			WorkspaceID: s.workspaceID,
			ScanJobID:   s.scanJobID,
			Phase:       s.phase,
			ToolName:    s.toolName,
			Data:        map[string]any{"result_count": s.resultCount},
		})
	}
}
