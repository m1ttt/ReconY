-- 001_initial.sql
-- ReconX initial schema

PRAGMA journal_mode = WAL;
PRAGMA foreign_keys = ON;
PRAGMA busy_timeout = 5000;

-- ============================================================
-- WORKSPACES
-- ============================================================
CREATE TABLE IF NOT EXISTS workspaces (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    domain      TEXT NOT NULL,
    description TEXT DEFAULT '',
    config_json TEXT DEFAULT '{}',
    created_at  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    updated_at  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_workspaces_domain ON workspaces(domain);

-- ============================================================
-- WORKFLOWS
-- ============================================================
CREATE TABLE IF NOT EXISTS workflows (
    id           TEXT PRIMARY KEY,
    workspace_id TEXT REFERENCES workspaces(id) ON DELETE CASCADE,
    name         TEXT NOT NULL,
    description  TEXT DEFAULT '',
    config       TEXT NOT NULL,
    is_builtin   INTEGER DEFAULT 0,
    created_at   TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    updated_at   TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
);
CREATE INDEX IF NOT EXISTS idx_workflows_workspace ON workflows(workspace_id);

-- ============================================================
-- SCAN JOBS
-- ============================================================
CREATE TABLE IF NOT EXISTS scan_jobs (
    id            TEXT PRIMARY KEY,
    workspace_id  TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    workflow_id   TEXT REFERENCES workflows(id),
    phase         INTEGER NOT NULL,
    tool_name     TEXT NOT NULL,
    status        TEXT NOT NULL DEFAULT 'queued',
    started_at    TEXT,
    finished_at   TEXT,
    result_count  INTEGER DEFAULT 0,
    error_message TEXT,
    config_json   TEXT DEFAULT '{}',
    created_at    TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
);
CREATE INDEX IF NOT EXISTS idx_scan_jobs_workspace ON scan_jobs(workspace_id, phase);
CREATE INDEX IF NOT EXISTS idx_scan_jobs_status ON scan_jobs(status);

-- ============================================================
-- TOOL LOGS
-- ============================================================
CREATE TABLE IF NOT EXISTS tool_logs (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    scan_job_id TEXT NOT NULL REFERENCES scan_jobs(id) ON DELETE CASCADE,
    stream      TEXT NOT NULL,
    line        TEXT NOT NULL,
    timestamp   TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%f', 'now'))
);
CREATE INDEX IF NOT EXISTS idx_tool_logs_job ON tool_logs(scan_job_id);

-- ============================================================
-- WHOIS / ASN
-- ============================================================
CREATE TABLE IF NOT EXISTS whois_records (
    id            TEXT PRIMARY KEY,
    workspace_id  TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    domain        TEXT NOT NULL,
    registrar     TEXT,
    org           TEXT,
    country       TEXT,
    creation_date TEXT,
    expiry_date   TEXT,
    name_servers  TEXT,
    raw           TEXT,
    asn           TEXT,
    asn_org       TEXT,
    asn_cidr      TEXT,
    scan_job_id   TEXT REFERENCES scan_jobs(id),
    created_at    TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
);
CREATE INDEX IF NOT EXISTS idx_whois_workspace ON whois_records(workspace_id);

-- ============================================================
-- DNS RECORDS
-- ============================================================
CREATE TABLE IF NOT EXISTS dns_records (
    id           TEXT PRIMARY KEY,
    workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    host         TEXT NOT NULL,
    record_type  TEXT NOT NULL,
    value        TEXT NOT NULL,
    ttl          INTEGER,
    priority     INTEGER,
    scan_job_id  TEXT REFERENCES scan_jobs(id),
    created_at   TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
);
CREATE INDEX IF NOT EXISTS idx_dns_workspace ON dns_records(workspace_id);
CREATE INDEX IF NOT EXISTS idx_dns_host ON dns_records(host, record_type);

-- ============================================================
-- HISTORICAL URLS
-- ============================================================
CREATE TABLE IF NOT EXISTS historical_urls (
    id           TEXT PRIMARY KEY,
    workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    url          TEXT NOT NULL,
    source       TEXT NOT NULL,
    scan_job_id  TEXT REFERENCES scan_jobs(id),
    created_at   TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
);
CREATE INDEX IF NOT EXISTS idx_histurls_workspace ON historical_urls(workspace_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_histurls_dedup ON historical_urls(workspace_id, url);

-- ============================================================
-- SUBDOMAINS
-- ============================================================
CREATE TABLE IF NOT EXISTS subdomains (
    id           TEXT PRIMARY KEY,
    workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    hostname     TEXT NOT NULL,
    ip_addresses TEXT,
    is_alive     INTEGER DEFAULT 0,
    source       TEXT NOT NULL,
    first_seen   TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    last_seen    TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    scan_job_id  TEXT REFERENCES scan_jobs(id)
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_subdomains_dedup ON subdomains(workspace_id, hostname);
CREATE INDEX IF NOT EXISTS idx_subdomains_alive ON subdomains(workspace_id, is_alive);

-- ============================================================
-- PORTS
-- ============================================================
CREATE TABLE IF NOT EXISTS ports (
    id           TEXT PRIMARY KEY,
    workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    subdomain_id TEXT REFERENCES subdomains(id) ON DELETE CASCADE,
    ip_address   TEXT NOT NULL,
    port         INTEGER NOT NULL,
    protocol     TEXT NOT NULL DEFAULT 'tcp',
    state        TEXT NOT NULL,
    service      TEXT,
    version      TEXT,
    banner       TEXT,
    scan_job_id  TEXT REFERENCES scan_jobs(id),
    created_at   TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_ports_dedup ON ports(workspace_id, ip_address, port, protocol);
CREATE INDEX IF NOT EXISTS idx_ports_subdomain ON ports(subdomain_id);

-- ============================================================
-- TECHNOLOGIES
-- ============================================================
CREATE TABLE IF NOT EXISTS technologies (
    id           TEXT PRIMARY KEY,
    workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    subdomain_id TEXT REFERENCES subdomains(id) ON DELETE CASCADE,
    url          TEXT NOT NULL,
    name         TEXT NOT NULL,
    version      TEXT,
    category     TEXT,
    confidence   INTEGER DEFAULT 100,
    scan_job_id  TEXT REFERENCES scan_jobs(id),
    created_at   TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
);
CREATE INDEX IF NOT EXISTS idx_tech_workspace ON technologies(workspace_id);
CREATE INDEX IF NOT EXISTS idx_tech_subdomain ON technologies(subdomain_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_tech_dedup ON technologies(workspace_id, url, name);

-- ============================================================
-- SITE CLASSIFICATIONS
-- ============================================================
CREATE TABLE IF NOT EXISTS site_classifications (
    id              TEXT PRIMARY KEY,
    workspace_id    TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    subdomain_id    TEXT REFERENCES subdomains(id) ON DELETE CASCADE,
    url             TEXT NOT NULL,
    site_type       TEXT NOT NULL,
    infra_type      TEXT,
    waf_detected    TEXT,
    cdn_detected    TEXT,
    ssl_grade       TEXT,
    ssl_details     TEXT,
    evidence        TEXT,
    scan_job_id     TEXT REFERENCES scan_jobs(id),
    created_at      TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_siteclass_dedup ON site_classifications(workspace_id, url);

-- ============================================================
-- DISCOVERED URLS
-- ============================================================
CREATE TABLE IF NOT EXISTS discovered_urls (
    id             TEXT PRIMARY KEY,
    workspace_id   TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    subdomain_id   TEXT REFERENCES subdomains(id) ON DELETE CASCADE,
    url            TEXT NOT NULL,
    status_code    INTEGER,
    content_type   TEXT,
    content_length INTEGER,
    source         TEXT NOT NULL,
    scan_job_id    TEXT REFERENCES scan_jobs(id),
    created_at     TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
);
CREATE INDEX IF NOT EXISTS idx_discurls_workspace ON discovered_urls(workspace_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_discurls_dedup ON discovered_urls(workspace_id, url);

-- ============================================================
-- PARAMETERS
-- ============================================================
CREATE TABLE IF NOT EXISTS parameters (
    id           TEXT PRIMARY KEY,
    workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    url          TEXT NOT NULL,
    name         TEXT NOT NULL,
    param_type   TEXT NOT NULL,
    source       TEXT NOT NULL,
    scan_job_id  TEXT REFERENCES scan_jobs(id),
    created_at   TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
);
CREATE INDEX IF NOT EXISTS idx_params_workspace ON parameters(workspace_id);

-- ============================================================
-- SCREENSHOTS
-- ============================================================
CREATE TABLE IF NOT EXISTS screenshots (
    id           TEXT PRIMARY KEY,
    workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    subdomain_id TEXT REFERENCES subdomains(id) ON DELETE CASCADE,
    url          TEXT NOT NULL,
    file_path    TEXT NOT NULL,
    status_code  INTEGER,
    title        TEXT,
    scan_job_id  TEXT REFERENCES scan_jobs(id),
    created_at   TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
);
CREATE INDEX IF NOT EXISTS idx_screenshots_workspace ON screenshots(workspace_id);

-- ============================================================
-- CLOUD ASSETS
-- ============================================================
CREATE TABLE IF NOT EXISTS cloud_assets (
    id           TEXT PRIMARY KEY,
    workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    provider     TEXT NOT NULL,
    asset_type   TEXT NOT NULL,
    name         TEXT NOT NULL,
    url          TEXT,
    is_public    INTEGER DEFAULT 0,
    permissions  TEXT,
    scan_job_id  TEXT REFERENCES scan_jobs(id),
    created_at   TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
);
CREATE INDEX IF NOT EXISTS idx_cloud_workspace ON cloud_assets(workspace_id);

-- ============================================================
-- SECRETS
-- ============================================================
CREATE TABLE IF NOT EXISTS secrets (
    id           TEXT PRIMARY KEY,
    workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    source_url   TEXT NOT NULL,
    secret_type  TEXT NOT NULL,
    value        TEXT NOT NULL,
    context      TEXT,
    source       TEXT NOT NULL,
    severity     TEXT NOT NULL DEFAULT 'medium',
    scan_job_id  TEXT REFERENCES scan_jobs(id),
    created_at   TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
);
CREATE INDEX IF NOT EXISTS idx_secrets_workspace ON secrets(workspace_id);

-- ============================================================
-- VULNERABILITIES
-- ============================================================
CREATE TABLE IF NOT EXISTS vulnerabilities (
    id            TEXT PRIMARY KEY,
    workspace_id  TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    subdomain_id  TEXT REFERENCES subdomains(id) ON DELETE CASCADE,
    template_id   TEXT NOT NULL,
    name          TEXT NOT NULL,
    severity      TEXT NOT NULL,
    url           TEXT NOT NULL,
    matched_at    TEXT,
    description   TEXT,
    reference     TEXT,
    curl_command  TEXT,
    extracted     TEXT,
    scan_job_id   TEXT REFERENCES scan_jobs(id),
    created_at    TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
);
CREATE INDEX IF NOT EXISTS idx_vulns_workspace ON vulnerabilities(workspace_id);
CREATE INDEX IF NOT EXISTS idx_vulns_severity ON vulnerabilities(workspace_id, severity);
CREATE UNIQUE INDEX IF NOT EXISTS idx_vulns_dedup ON vulnerabilities(workspace_id, template_id, url);
