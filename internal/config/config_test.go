package config

import (
	"path/filepath"
	"testing"
)

func TestDefaults(t *testing.T) {
	cfg, err := Load(filepath.Join(t.TempDir(), "missing.yaml"))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.Disguise.Port != 80 || cfg.Disguise.Host != "127.0.0.1" {
		t.Fatalf("disguise defaults wrong: %+v", cfg.Disguise)
	}
	if cfg.MCP.Transport != TransportHTTP || cfg.MCP.HTTP.Addr != "0.0.0.0:8090" {
		t.Fatalf("mcp defaults wrong: %+v", cfg.MCP)
	}
	if cfg.Target().BaseURL() != "http://127.0.0.1:80" {
		t.Fatalf("target = %q", cfg.Target().BaseURL())
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cfg.yaml")
	cfg := Default()
	cfg.Disguise.Host = "10.0.0.20"
	cfg.Disguise.Port = 8000
	if err := cfg.Save(path); err != nil {
		t.Fatalf("save: %v", err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded.Disguise.Host != "10.0.0.20" || loaded.Disguise.Port != 8000 {
		t.Fatalf("round-trip lost values: %+v", loaded.Disguise)
	}
}

func TestValidate(t *testing.T) {
	cfg := Default()
	cfg.MCP.Transport = "bogus"
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected transport error")
	}
	cfg = Default()
	cfg.Disguise.Scheme = "ftp"
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected scheme error")
	}
}
