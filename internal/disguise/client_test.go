package disguise

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"
	"time"
)

// targetFor builds a Target aimed at an httptest server.
func targetFor(t *testing.T, srv *httptest.Server) Target {
	t.Helper()
	u, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatalf("parse url: %v", err)
	}
	port, _ := strconv.Atoi(u.Port())
	return Target{Scheme: u.Scheme, Host: u.Hostname(), Port: port}
}

func TestPostSendsBodyAndPath(t *testing.T) {
	var gotPath string
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &gotBody)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer srv.Close()

	c := NewClient(targetFor(t, srv), 5*time.Second)
	raw, err := c.Post(context.Background(), "/api/session/transport/play", map[string]any{"transports": []any{map[string]any{"name": "Main"}}})
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	if gotPath != "/api/session/transport/play" {
		t.Fatalf("path = %q", gotPath)
	}
	if _, ok := gotBody["transports"]; !ok {
		t.Fatalf("body missing transports: %v", gotBody)
	}
	if string(raw) != `{"status":"ok"}` {
		t.Fatalf("unexpected response: %s", raw)
	}
}

func TestErrorOnNon2xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "no such transport", http.StatusNotFound)
	}))
	defer srv.Close()
	c := NewClient(targetFor(t, srv), 5*time.Second)
	if _, err := c.Get(context.Background(), "/api/service/system/osinfo"); err == nil {
		t.Fatal("expected error on 404")
	}
}

func TestSetTargetRepoints(t *testing.T) {
	c := NewClient(Target{Host: "10.0.0.1"}, time.Second)
	if got := c.Target().BaseURL(); got != "http://10.0.0.1:80" {
		t.Fatalf("default base = %q", got)
	}
	c.SetTarget(Target{Scheme: "https", Host: "10.0.0.2", Port: 443})
	if got := c.Target().BaseURL(); got != "https://10.0.0.2:443" {
		t.Fatalf("repointed base = %q", got)
	}
}
