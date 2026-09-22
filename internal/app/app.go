// Package app holds the shared runtime state — the disguise client and current
// configuration — used by both the MCP server and the admin web console, so a
// target change from either side is applied and persisted consistently.
package app

import (
	"context"
	"sync"
	"time"

	"github.com/medcelerate/disguise-mcp/internal/config"
	"github.com/medcelerate/disguise-mcp/internal/disguise"
)

// App is the central handle to the disguise client and config.
type App struct {
	client  *disguise.Client
	live    *disguise.LiveClient
	logf    func(string, ...any)
	mu      sync.Mutex
	cfg     *config.Config
	cfgPath string
}

// New builds an App from a config, aiming the disguise client at the configured
// target. cfgPath may be empty (changes then apply in memory only).
func New(cfg *config.Config, cfgPath string, logf func(string, ...any)) *App {
	if logf == nil {
		logf = func(string, ...any) {}
	}
	timeout := time.Duration(cfg.Disguise.TimeoutSeconds) * time.Second
	a := &App{
		client:  disguise.NewClient(cfg.Target(), timeout),
		logf:    logf,
		cfg:     cfg,
		cfgPath: cfgPath,
	}
	a.live = disguise.NewLiveClient(a.Target, logf)
	return a
}

// Client returns the disguise API client.
func (a *App) Client() *disguise.Client { return a.client }

// Live returns the Live Update (WebSocket) client.
func (a *App) Live() *disguise.LiveClient { return a.live }

// Config returns the current configuration.
func (a *App) Config() *config.Config {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.cfg
}

// Target returns the disguise server the client currently points at.
func (a *App) Target() disguise.Target { return a.client.Target() }

// SetTarget repoints the disguise client and persists the change to the config
// file (when a path is configured).
func (a *App) SetTarget(t disguise.Target) error {
	a.client.SetTarget(t)
	a.mu.Lock()
	defer a.mu.Unlock()
	nt := a.client.Target()
	a.cfg.Disguise.Scheme = nt.Scheme
	a.cfg.Disguise.Host = nt.Host
	a.cfg.Disguise.Port = nt.Port
	a.logf("disguise target set to %s", nt.BaseURL())
	if a.live != nil {
		a.live.Reset() // drop the live-update socket; it redials the new target
	}
	if a.cfgPath == "" {
		return nil
	}
	return a.cfg.Save(a.cfgPath)
}

// Status is a snapshot of the connection to the disguise server.
type Status struct {
	Target    disguise.Target `json:"target"`
	BaseURL   string          `json:"baseURL"`
	Reachable bool            `json:"reachable"`
	Error     string          `json:"error,omitempty"`
}

// Status pings the disguise server and reports reachability.
func (a *App) Status(ctx context.Context) Status {
	t := a.Target()
	st := Status{Target: t, BaseURL: t.BaseURL()}
	if err := a.client.Ping(ctx); err != nil {
		st.Error = err.Error()
		return st
	}
	st.Reachable = true
	return st
}
