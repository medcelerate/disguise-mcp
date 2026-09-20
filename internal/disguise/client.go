// Package disguise is an HTTP client for the disguise Designer API
// (https://developer.disguise.one/api/). The API is a REST/JSON service that
// runs on the disguise server (default port 80) exposing Service endpoints
// (system, project, media, tasks) and Session endpoints (transport, etc.).
//
// The client's target (host/port/scheme) is repointable at runtime so the
// bridge can be aimed at a different disguise server without a restart.
package disguise

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Target identifies a disguise server's API endpoint.
type Target struct {
	Scheme string `json:"scheme" yaml:"scheme"`
	Host   string `json:"host" yaml:"host"`
	Port   int    `json:"port" yaml:"port"`
}

// withDefaults fills unset fields with disguise API defaults.
func (t Target) withDefaults() Target {
	if t.Scheme == "" {
		t.Scheme = "http"
	}
	if t.Port == 0 {
		t.Port = 80
	}
	return t
}

// BaseURL renders the target as an origin URL, e.g. http://10.0.0.5:80.
func (t Target) BaseURL() string {
	t = t.withDefaults()
	return fmt.Sprintf("%s://%s:%d", t.Scheme, t.Host, t.Port)
}

// Client talks to a disguise Designer API. It is safe for concurrent use, and
// its target can be changed while requests are in flight.
type Client struct {
	target atomic.Pointer[Target]
	http   *http.Client
	mu     sync.Mutex // serialises target swaps for a consistent read-modify
}

// NewClient creates a client aimed at target with the given request timeout.
func NewClient(target Target, timeout time.Duration) *Client {
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	c := &Client{http: &http.Client{Timeout: timeout}}
	t := target.withDefaults()
	c.target.Store(&t)
	return c
}

// Target returns the current target.
func (c *Client) Target() Target { return *c.target.Load() }

// SetTarget repoints the client at a new disguise server.
func (c *Client) SetTarget(t Target) {
	c.mu.Lock()
	defer c.mu.Unlock()
	t = t.withDefaults()
	c.target.Store(&t)
}

// Do performs an API request. path is the API path (e.g.
// "/api/session/transport/play"); body is marshalled as JSON when non-nil.
// The decoded response body is returned as raw JSON (or empty for no content).
func (c *Client) Do(ctx context.Context, method, path string, body any) (json.RawMessage, error) {
	target := c.Target()
	if target.Host == "" {
		return nil, fmt.Errorf("no disguise target configured")
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	url := target.BaseURL() + path

	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal request: %w", err)
		}
		reader = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, url, reader)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%s %s: %w", method, path, err)
	}
	defer func() { _ = resp.Body.Close() }()

	data, err := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("%s %s: disguise returned %s: %s", method, path, resp.Status, strings.TrimSpace(string(data)))
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return json.RawMessage("null"), nil
	}
	return json.RawMessage(data), nil
}

// Get is a convenience wrapper for GET requests.
func (c *Client) Get(ctx context.Context, path string) (json.RawMessage, error) {
	return c.Do(ctx, http.MethodGet, path, nil)
}

// Post is a convenience wrapper for POST requests with a JSON body.
func (c *Client) Post(ctx context.Context, path string, body any) (json.RawMessage, error) {
	return c.Do(ctx, http.MethodPost, path, body)
}

// Ping checks reachability by requesting a lightweight Service endpoint (OS
// info, always available). It returns nil if the disguise server responds.
func (c *Client) Ping(ctx context.Context) error {
	_, err := c.Get(ctx, "/api/service/system/osinfo")
	return err
}
