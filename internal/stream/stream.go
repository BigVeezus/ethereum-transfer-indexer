package stream

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"pagrin/internal/models"
	"pagrin/pkg/logger"
)

// Stream provides real-time event streaming to connected clients
// Supports both WebSocket and Server-Sent Events (SSE)
type Stream struct {
	clients    map[chan []byte]bool
	mu         sync.RWMutex
	buffer     []*models.Transfer
	bufferSize int
	logger     *logger.Logger
}

// NewStream creates a new stream instance
func NewStream(bufferSize int, log *logger.Logger) *Stream {
	return &Stream{
		clients:    make(map[chan []byte]bool),
		buffer:     make([]*models.Transfer, 0, bufferSize),
		bufferSize: bufferSize,
		logger:     log,
	}
}

// Publish sends a transfer event to all connected clients
// Non-blocking: if client channel is full, event is dropped for that client
// Accepts interface{} to match StreamPublisher interface
func (s *Stream) Publish(transfer interface{}) {
	// Defensive nil check - should never happen but prevents panic
	if s == nil {
		return
	}

	// Type assert to *models.Transfer
	t, ok := transfer.(*models.Transfer)
	if !ok {
		s.logger.Error("Invalid transfer type for streaming")
		return
	}
	s.mu.RLock()
	defer s.mu.RUnlock()

	if len(s.clients) == 0 {
		// No clients connected - buffer the event
		if len(s.buffer) < s.bufferSize {
			s.buffer = append(s.buffer, t)
		}
		return
	}

	// Serialize transfer to JSON
	data, err := json.Marshal(t)
	if err != nil {
		s.logger.Error("Failed to marshal transfer for streaming: %v", err)
		return
	}

	// Send to all connected clients (non-blocking)
	for clientChan := range s.clients {
		select {
		case clientChan <- data:
			// Successfully sent
		default:
			// Channel full - skip this client (prevents blocking)
			s.logger.Debug("Client channel full, dropping event")
		}
	}
}

// Subscribe creates a new client channel for receiving events
// Returns channel and cleanup function
func (s *Stream) Subscribe() (chan []byte, func()) {
	s.mu.Lock()
	defer s.mu.Unlock()

	clientChan := make(chan []byte, s.bufferSize)

	// Send buffered events to new client
	go func() {
		s.mu.RLock()
		buffered := make([]*models.Transfer, len(s.buffer))
		copy(buffered, s.buffer)
		s.mu.RUnlock()

		for _, transfer := range buffered {
			data, err := json.Marshal(transfer)
			if err != nil {
				continue
			}
			select {
			case clientChan <- data:
			case <-time.After(1 * time.Second):
				// Timeout - client may have disconnected
				return
			}
		}
	}()

	s.clients[clientChan] = true

	cleanup := func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		delete(s.clients, clientChan)
		close(clientChan)
	}

	return clientChan, cleanup
}

// ClientCount returns the number of connected clients
func (s *Stream) ClientCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.clients)
}

// ClearBuffer clears the event buffer (useful for memory management)
func (s *Stream) ClearBuffer() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.buffer = make([]*models.Transfer, 0, s.bufferSize)
}

// StartBackgroundCleanup periodically clears old buffered events
// Prevents memory growth if no clients connect for extended periods
func (s *Stream) StartBackgroundCleanup(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Clear buffer if no clients and buffer is old
			if s.ClientCount() == 0 && len(s.buffer) > 0 {
				s.mu.Lock()
				if len(s.buffer) > s.bufferSize/2 {
					// Keep only recent half
					s.buffer = s.buffer[len(s.buffer)/2:]
				}
				s.mu.Unlock()
			}
		}
	}
}
