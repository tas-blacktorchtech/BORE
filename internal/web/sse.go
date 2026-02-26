package web

import (
	"fmt"
	"log"
	"net/http"
	"time"
)

// sseHub manages Server-Sent Events clients.
type sseHub struct {
	subscribe   chan chan string
	unsubscribe chan chan string
	broadcast   chan string
	quit        chan struct{}
	clients     map[chan string]struct{}
}

// newSSEHub returns a new, uninitialised sseHub. Call run() in a goroutine.
func newSSEHub() *sseHub {
	return &sseHub{
		subscribe:   make(chan chan string, 8),
		unsubscribe: make(chan chan string, 8),
		broadcast:   make(chan string, 256),
		quit:        make(chan struct{}),
		clients:     make(map[chan string]struct{}),
	}
}

// run is the event loop for the hub. Must be called in a dedicated goroutine.
// The run goroutine is the sole owner of clients; no mutex is needed.
func (h *sseHub) run() {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case ch := <-h.subscribe:
			h.clients[ch] = struct{}{}

		case ch := <-h.unsubscribe:
			delete(h.clients, ch)
			close(ch)

		case msg := <-h.broadcast:
			for ch := range h.clients {
				select {
				case ch <- msg:
				default:
				}
			}

		case <-ticker.C:
			ping := "event: ping\ndata: {}\n\n"
			for ch := range h.clients {
				select {
				case ch <- ping:
				default:
				}
			}

		case <-h.quit:
			return
		}
	}
}

// emit sends a named SSE event with a JSON data payload to all connected clients.
// Uses a non-blocking send so it never blocks if the broadcast channel is full.
func (h *sseHub) emit(event, data string) {
	msg := fmt.Sprintf("event: %s\ndata: %s\n\n", event, data)
	select {
	case h.broadcast <- msg:
	default:
		log.Printf("[sse] WARNING: broadcast buffer full, dropping event: %s", event)
	}
}

// handleSSE is the HTTP handler for the /events SSE endpoint.
func (s *Server) handleSSE(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	// Allow localhost browser access; no auth on this server.
	w.Header().Set("Access-Control-Allow-Origin", "*")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	ch := make(chan string, 32)
	s.hub.subscribe <- ch
	defer func() { s.hub.unsubscribe <- ch }()

	if _, err := fmt.Fprintf(w, "event: connected\ndata: {}\n\n"); err != nil {
		return
	}
	flusher.Flush()

	for {
		select {
		case msg, open := <-ch:
			if !open {
				return
			}
			fmt.Fprint(w, msg)
			flusher.Flush()
		case <-r.Context().Done():
			return
		}
	}
}
