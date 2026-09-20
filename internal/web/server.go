// Package web serves the embedded admin console: a single-page UI plus a small
// JSON API for viewing status and repointing the bridge at a different disguise
// server. It runs on a separate port from the MCP endpoint.
package web

import (
	"context"
	"embed"
	"io/fs"
	"net/http"
	"time"

	"github.com/medcelerate/disguise-mcp/internal/app"
)

//go:embed static/*
var staticFS embed.FS

// Server is the admin console HTTP server.
type Server struct {
	app  *app.App
	logf func(string, ...any)
}

// New creates an admin server.
func New(a *app.App, logf func(string, ...any)) *Server {
	if logf == nil {
		logf = func(string, ...any) {}
	}
	return &Server{app: a, logf: logf}
}

// Handler returns the HTTP handler serving the console and API.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	sub, _ := fs.Sub(staticFS, "static")
	mux.Handle("/", http.FileServer(http.FS(sub)))
	mux.HandleFunc("/api/status", s.handleStatus)
	mux.HandleFunc("/api/target", s.handleTarget)
	return mux
}

// Serve runs the admin server on addr until ctx is cancelled.
func (s *Server) Serve(ctx context.Context, addr string) error {
	srv := &http.Server{
		Addr:              addr,
		Handler:           s.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}
	go func() {
		<-ctx.Done()
		shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutCtx)
	}()
	s.logf("admin console listening on http://%s", addr)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}
