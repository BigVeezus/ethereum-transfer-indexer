package handler

import (
	"net/http"
	"time"

	"pagrin/internal/stream"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for MVP
	},
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

// StreamHandler handles WebSocket and SSE streaming connections
type StreamHandler struct {
	stream *stream.Stream
}

// NewStreamHandler creates a new stream handler
func NewStreamHandler(s *stream.Stream) *StreamHandler {
	return &StreamHandler{stream: s}
}

// HandleWebSocket handles WebSocket connections for real-time transfer events
// Clients receive JSON-encoded transfer events as they are processed
func (h *StreamHandler) HandleWebSocket(c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to upgrade connection"})
		return
	}
	defer conn.Close()

	// Subscribe to stream
	clientChan, cleanup := h.stream.Subscribe()
	defer cleanup()

	// Send ping periodically to keep connection alive
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	// Set read deadline to detect client disconnection
	conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	// Send events to client
	for {
		select {
		case data, ok := <-clientChan:
			if !ok {
				// Channel closed
				return
			}

			conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
				return // Client disconnected
			}

		case <-ticker.C:
			// Send ping
			conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// HandleSSE handles Server-Sent Events for real-time transfer events
// SSE is simpler than WebSocket but only supports server-to-client communication
func (h *StreamHandler) HandleSSE(c *gin.Context) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")

	// Subscribe to stream
	clientChan, cleanup := h.stream.Subscribe()
	defer cleanup()

	// Send events to client
	for {
		select {
		case data, ok := <-clientChan:
			if !ok {
				// Channel closed
				return
			}

			// SSE format: "data: <json>\n\n"
			c.SSEvent("transfer", string(data))
			c.Writer.Flush()

		case <-c.Request.Context().Done():
			// Client disconnected
			return
		}
	}
}
