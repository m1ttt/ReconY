-- 002_auth_credentials.sql
-- Workspace-scoped authentication credentials for authenticated crawling

CREATE TABLE IF NOT EXISTS auth_credentials (
    id            TEXT PRIMARY KEY,
    workspace_id  TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    name          TEXT NOT NULL,
    auth_type     TEXT NOT NULL DEFAULT 'none',
    username      TEXT,
    password      TEXT,
    login_url     TEXT,
    login_body    TEXT,
    token         TEXT,
    header_name   TEXT,
    header_value  TEXT,
    is_active     INTEGER DEFAULT 1,
    created_at    TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    updated_at    TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
);
CREATE INDEX IF NOT EXISTS idx_authcreds_workspace ON auth_credentials(workspace_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_authcreds_dedup ON auth_credentials(workspace_id, name);

-- Track auth-only findings
ALTER TABLE discovered_urls ADD COLUMN requires_auth INTEGER DEFAULT 0;
ALTER TABLE discovered_urls ADD COLUMN auth_credential_id TEXT REFERENCES auth_credentials(id);
