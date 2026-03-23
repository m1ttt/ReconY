package engine

import (
	"sync"
	"time"
)

// EventType represents the type of engine event.
type EventType string

const (
	EventScanStarted    EventType = "scan.started"
	EventScanProgress   EventType = "scan.progress"
	EventScanCompleted  EventType = "scan.completed"
	EventScanFailed     EventType = "scan.failed"
	EventScanCancelled  EventType = "scan.cancelled"
	EventScanLogLine    EventType = "scan.log_line"
	EventPhaseStarted   EventType = "phase.started"
	EventPhaseCompleted EventType = "phase.completed"
	EventNewSubdomain   EventType = "result.new_subdomain"
	EventNewPort        EventType = "result.new_port"
	EventNewTech        EventType = "result.new_tech"
	EventNewVuln        EventType = "result.new_vuln"
	EventNewSecret      EventType = "result.new_secret"
	EventNewURL         EventType = "result.new_url"
	EventNewScreenshot  EventType = "result.new_screenshot"
	EventNewCloudAsset  EventType = "result.new_cloud_asset"
)

// Event is emitted by the engine during scan execution.
type Event struct {
	Type        EventType   `json:"type"`
	WorkspaceID string      `json:"workspace_id"`
	ScanJobID   string      `json:"scan_job_id,omitempty"`
	Phase       int         `json:"phase,omitempty"`
	ToolName    string      `json:"tool_name,omitempty"`
	Data        any         `json:"data,omitempty"`
	Timestamp   time.Time   `json:"timestamp"`
}

// EventBus is a pub/sub system for engine events.
// Subscribers can listen for events from a specific workspace or all workspaces.
type EventBus struct {
	mu          sync.RWMutex
	subscribers map[string][]chan Event // key: workspace_id ("" for global)
}

// NewEventBus creates a new event bus.
func NewEventBus() *EventBus {
	return &EventBus{
		subscribers: make(map[string][]chan Event),
	}
}

// Subscribe returns a channel that receives events for the given workspace.
// Pass "" for workspaceID to receive all events.
func (eb *EventBus) Subscribe(workspaceID string) chan Event {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	ch := make(chan Event, 256)
	eb.subscribers[workspaceID] = append(eb.subscribers[workspaceID], ch)
	return ch
}

// Unsubscribe removes a subscriber channel.
func (eb *EventBus) Unsubscribe(workspaceID string, ch chan Event) {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	subs := eb.subscribers[workspaceID]
	for i, sub := range subs {
		if sub == ch {
			eb.subscribers[workspaceID] = append(subs[:i], subs[i+1:]...)
			close(ch)
			return
		}
	}
}

// Publish sends an event to all matching subscribers.
func (eb *EventBus) Publish(event Event) {
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now().UTC()
	}

	eb.mu.RLock()
	defer eb.mu.RUnlock()

	// Send to workspace-specific subscribers
	for _, ch := range eb.subscribers[event.WorkspaceID] {
		select {
		case ch <- event:
		default:
			// Drop if subscriber is slow
		}
	}

	// Send to global subscribers
	if event.WorkspaceID != "" {
		for _, ch := range eb.subscribers[""] {
			select {
			case ch <- event:
			default:
			}
		}
	}
}
