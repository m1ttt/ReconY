package ws

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"reconx/internal/engine"
	"nhooyr.io/websocket"
	"nhooyr.io/websocket/wsjson"
)

// Hub manages WebSocket connections and bridges them to the EventBus.
type Hub struct {
	eventBus *engine.EventBus
	mu       sync.Mutex
	clients  map[*client]bool
}

type client struct {
	conn        *websocket.Conn
	workspaceID string
	send        chan engine.Event
}

// NewHub creates a new WebSocket hub.
func NewHub(eventBus *engine.EventBus) *Hub {
	return &Hub{
		eventBus: eventBus,
		clients:  make(map[*client]bool),
	}
}

// ServeHTTP handles WebSocket upgrade and connection.
func (h *Hub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	wsConn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		OriginPatterns: []string{"localhost:*", "127.0.0.1:*"},
	})
	if err != nil {
		log.Printf("ws accept error: %v", err)
		return
	}

	workspaceID := r.URL.Query().Get("workspace_id")

	c := &client{
		conn:        wsConn,
		workspaceID: workspaceID,
		send:        make(chan engine.Event, 256),
	}

	h.mu.Lock()
	h.clients[c] = true
	h.mu.Unlock()

	// Subscribe to events
	eventCh := h.eventBus.Subscribe(workspaceID)

	ctx := r.Context()

	// Forward events to client
	go func() {
		defer func() {
			h.eventBus.Unsubscribe(workspaceID, eventCh)
			h.mu.Lock()
			delete(h.clients, c)
			h.mu.Unlock()
			wsConn.Close(websocket.StatusNormalClosure, "")
		}()

		for {
			select {
			case event, ok := <-eventCh:
				if !ok {
					return
				}
				writeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
				err := wsjson.Write(writeCtx, wsConn, event)
				cancel()
				if err != nil {
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	// Read loop (handle client messages like subscribe changes)
	for {
		_, data, err := wsConn.Read(ctx)
		if err != nil {
			break
		}

		var msg struct {
			Type        string `json:"type"`
			WorkspaceID string `json:"workspace_id"`
		}
		if json.Unmarshal(data, &msg) == nil {
			if msg.Type == "subscribe" && msg.WorkspaceID != "" {
				// Re-subscribe to different workspace
				h.eventBus.Unsubscribe(workspaceID, eventCh)
				workspaceID = msg.WorkspaceID
				eventCh = h.eventBus.Subscribe(workspaceID)
			}
		}
	}
}

// ActiveConnections returns the number of active WebSocket clients.
func (h *Hub) ActiveConnections() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.clients)
}
