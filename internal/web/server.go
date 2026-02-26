// Package web provides the BORE web GUI HTTP server with REST API and SSE.
package web

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"time"

	"bore/internal/app"
)

//go:embed static
var staticFiles embed.FS

// DefaultPort is the first port tried when auto-assigning.
const DefaultPort = 8742

// Server is the BORE web GUI HTTP server.
type Server struct {
	a        *app.App
	srv      *http.Server
	port     int
	bindAddr string
	hub      *sseHub
}

// New creates a new Server bound to the given App and bind address.
// bindAddr should be "127.0.0.1" for localhost-only or "0.0.0.0" for all
// interfaces (network mode).
func New(a *app.App, bindAddr string) *Server {
	if bindAddr == "" {
		bindAddr = "127.0.0.1"
	}
	return &Server{a: a, bindAddr: bindAddr, hub: newSSEHub()}
}

// Port returns the port the server is listening on (0 if not started).
func (s *Server) Port() int { return s.port }

// BindAddr returns the configured bind address.
func (s *Server) BindAddr() string { return s.bindAddr }

// NetworkMode returns true when the server is bound to all interfaces.
func (s *Server) NetworkMode() bool { return s.bindAddr == "0.0.0.0" }

// URL returns the base URL using the bind address host component.
// For "0.0.0.0" it uses "localhost" so the URL is browser-friendly.
func (s *Server) URL() string {
	host := s.bindAddr
	if host == "0.0.0.0" {
		host = "localhost"
	}
	return fmt.Sprintf("http://%s:%d", host, s.port)
}

// LocalIPs enumerates non-loopback IPv4 addresses on the machine.
func (s *Server) LocalIPs() []string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil
	}
	var ips []string
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 {
			continue
		}
		if iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if ip == nil || ip.IsLoopback() {
				continue
			}
			if ip4 := ip.To4(); ip4 != nil {
				ips = append(ips, ip4.String())
			}
		}
	}
	return ips
}

// Start binds to a free port starting at DefaultPort, starts the HTTP server
// in a background goroutine. Returns the URL.
func (s *Server) Start(ctx context.Context) (string, error) {
	return s.StartOnPort(ctx, DefaultPort)
}

// StartOnPort binds to a free port starting at startPort, starts the HTTP
// server in a background goroutine, and returns the URL.
func (s *Server) StartOnPort(ctx context.Context, startPort int) (string, error) {
	ln, err := freePort(s.bindAddr, startPort)
	if err != nil {
		return "", fmt.Errorf("web: start: find port: %w", err)
	}
	s.port = ln.Addr().(*net.TCPAddr).Port

	mux := http.NewServeMux()
	s.registerRoutes(mux)

	s.srv = &http.Server{
		Addr:              fmt.Sprintf("%s:%d", s.bindAddr, s.port),
		Handler:           corsMiddleware(mux),
		ReadTimeout:       30 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		// WriteTimeout intentionally 0 — SSE connections must not time out.
	}

	go func() {
		if err := s.srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			// error is non-fatal after Stop() is called; log to stderr in debug builds
			_ = err
		}
	}()
	go s.hub.run()

	// Allow the app layer to push SSE events directly (e.g. from execution runner).
	s.a.SetEventEmitter(s.hub.emit)

	return s.URL(), nil
}

// Stop gracefully shuts down the server.
func (s *Server) Stop(ctx context.Context) error {
	if s.srv == nil {
		return nil
	}
	if err := s.srv.Shutdown(ctx); err != nil {
		return fmt.Errorf("web: stop: %w", err)
	}
	close(s.hub.quit)
	return nil
}

func (s *Server) registerRoutes(mux *http.ServeMux) {
	static, err := fs.Sub(staticFiles, "static")
	if err != nil {
		panic(fmt.Sprintf("web: embed static: %v", err))
	}
	// SPA fallback: serve the file if it exists, otherwise serve index.html
	// so that React Router can handle client-side routes.
	fileServer := http.FileServer(http.FS(static))
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		// Try to open the requested file.
		path := r.URL.Path
		if path == "/" {
			fileServer.ServeHTTP(w, r)
			return
		}
		// Strip leading slash for fs.Open.
		name := path[1:]
		if f, err := static.Open(name); err == nil {
			f.Close()
			fileServer.ServeHTTP(w, r)
			return
		}
		// File not found — serve index.html for SPA routing.
		r.URL.Path = "/"
		fileServer.ServeHTTP(w, r)
	})

	// SSE
	mux.HandleFunc("GET /events", s.handleSSE)

	// Status & cluster management
	mux.HandleFunc("GET /api/status", s.handleGetStatus)
	mux.HandleFunc("GET /api/clusters", s.handleGetClusters)
	mux.HandleFunc("POST /api/clusters/open", s.handleOpenCluster)
	mux.HandleFunc("POST /api/clusters/init", s.handleClusterInit)
	mux.HandleFunc("POST /api/clusters/delete", s.handleDeleteCluster)

	// Tasks
	mux.HandleFunc("GET /api/tasks", s.handleListTasks)
	mux.HandleFunc("POST /api/tasks", s.handleCreateTask)
	mux.HandleFunc("GET /api/tasks/{id}", s.handleGetTask)
	mux.HandleFunc("GET /api/tasks/{id}/reviews", s.handleGetTaskReviews)
	mux.HandleFunc("POST /api/tasks/{id}/review", s.handleSubmitTaskReview)
	mux.HandleFunc("POST /api/tasks/{id}/clarifications", s.handleSubmitClarifications)

	// Executions
	mux.HandleFunc("GET /api/executions", s.handleListExecutions)
	mux.HandleFunc("GET /api/executions/{id}", s.handleGetExecution)
	mux.HandleFunc("GET /api/executions/{id}/events", s.handleListEvents)
	mux.HandleFunc("GET /api/executions/{id}/runs", s.handleListAgentRuns)

	// Diff actions
	mux.HandleFunc("GET /api/diff/{id}", s.handleGetDiff)
	mux.HandleFunc("POST /api/diff/{id}/commit", s.handleDiffCommit)
	mux.HandleFunc("POST /api/diff/{id}/revert", s.handleDiffRevert)
	mux.HandleFunc("POST /api/diff/{id}/merge", s.handleDiffMerge)

	// Crews
	mux.HandleFunc("GET /api/crews", s.handleListCrews)
	mux.HandleFunc("POST /api/crews", s.handleCreateCrew)
	mux.HandleFunc("PUT /api/crews/{id}", s.handleUpdateCrew)
	mux.HandleFunc("DELETE /api/crews/{id}", s.handleDeleteCrew)

	// Threads
	mux.HandleFunc("GET /api/threads", s.handleListThreads)
	mux.HandleFunc("POST /api/threads", s.handleCreateThread)

	// Brain
	mux.HandleFunc("GET /api/brain", s.handleGetBrain)
	mux.HandleFunc("PUT /api/brain", s.handleSaveBrain)
	mux.HandleFunc("POST /api/brain/scan", s.handleBrainScan)

	// Commander chat
	mux.HandleFunc("POST /api/commander/chat", s.handleCommanderChat)

	// Git
	mux.HandleFunc("GET /api/branches", s.handleListBranches)

	// Server info
	mux.HandleFunc("GET /api/server/info", s.handleServerInfo)
}

// corsMiddleware wraps a handler to set CORS headers and handle OPTIONS preflight.
// Needed for dev mode (Vite on :5173 → Go on :8742) and network mode (LAN devices).
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// freePort finds the first available TCP port starting from start on the given
// bindAddr and returns the bound listener. The caller is responsible for using
// or closing it.
func freePort(bindAddr string, start int) (net.Listener, error) {
	for p := start; p < start+100; p++ {
		ln, err := net.Listen("tcp", fmt.Sprintf("%s:%d", bindAddr, p))
		if err == nil {
			return ln, nil
		}
	}
	return nil, fmt.Errorf("web: freePort: no free port found in range %d-%d", start, start+100)
}
