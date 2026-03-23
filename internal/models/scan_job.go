package models

// ScanStatus represents the state of a scan job.
type ScanStatus string

const (
	ScanStatusQueued    ScanStatus = "queued"
	ScanStatusRunning   ScanStatus = "running"
	ScanStatusCompleted ScanStatus = "completed"
	ScanStatusFailed    ScanStatus = "failed"
	ScanStatusCancelled ScanStatus = "cancelled"
)

// ScanJob represents the execution of a single tool within a phase.
type ScanJob struct {
	ID           string     `json:"id" db:"id"`
	WorkspaceID  string     `json:"workspace_id" db:"workspace_id"`
	WorkflowID   *string    `json:"workflow_id,omitempty" db:"workflow_id"`
	Phase        int        `json:"phase" db:"phase"`
	ToolName     string     `json:"tool_name" db:"tool_name"`
	Status       ScanStatus `json:"status" db:"status"`
	StartedAt    *string    `json:"started_at,omitempty" db:"started_at"`
	FinishedAt   *string    `json:"finished_at,omitempty" db:"finished_at"`
	ResultCount  int        `json:"result_count" db:"result_count"`
	ErrorMessage *string    `json:"error_message,omitempty" db:"error_message"`
	ConfigJSON   string     `json:"config_json" db:"config_json"`
	CreatedAt    string     `json:"created_at" db:"created_at"`
}

// ToolLog represents a line of stdout/stderr from a tool execution.
type ToolLog struct {
	ID        int64  `json:"id" db:"id"`
	ScanJobID string `json:"scan_job_id" db:"scan_job_id"`
	Stream    string `json:"stream" db:"stream"`
	Line      string `json:"line" db:"line"`
	Timestamp string `json:"timestamp" db:"timestamp"`
}
