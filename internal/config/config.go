// Package config defines the disguise-mcp configuration file format and the
// loading, validation and persistence logic shared by the CLI and the web UI.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/medcelerate/disguise-mcp/internal/disguise"
	"gopkg.in/yaml.v3"
)

// MCP transport values.
const (
	TransportStdio = "stdio"
	TransportHTTP  = "http"
	TransportBoth  = "both"
)

// Config is the full bridge configuration.
type Config struct {
	Disguise DisguiseConfig `yaml:"disguise"`
	MCP      MCPConfig      `yaml:"mcp"`
	Web      WebConfig      `yaml:"web"`
	Log      LogConfig      `yaml:"log"`
}

// DisguiseConfig is the target disguise server's API endpoint.
type DisguiseConfig struct {
	Scheme         string `yaml:"scheme"`
	Host           string `yaml:"host"`
	Port           int    `yaml:"port"`
	TimeoutSeconds int    `yaml:"timeoutSeconds"`
}

// MCPConfig selects how MCP clients connect.
type MCPConfig struct {
	Transport string     `yaml:"transport"`
	HTTP      HTTPConfig `yaml:"http"`
}

// HTTPConfig is the bind address for the Streamable HTTP MCP endpoint.
type HTTPConfig struct {
	Addr string `yaml:"addr"`
}

// WebConfig controls the embedded admin console.
type WebConfig struct {
	Enabled bool   `yaml:"enabled"`
	Addr    string `yaml:"addr"`
}

// LogConfig controls logging.
type LogConfig struct {
	Level string `yaml:"level"`
}

// Default returns a configuration with sensible defaults filled in.
func Default() Config {
	return Config{
		Disguise: DisguiseConfig{Scheme: "http", Host: "127.0.0.1", Port: 80, TimeoutSeconds: 15},
		// MCP is exposed on all interfaces by default: the bridge runs on the
		// disguise server and AI clients connect to it over the network.
		MCP: MCPConfig{Transport: TransportHTTP, HTTP: HTTPConfig{Addr: "0.0.0.0:8090"}},
		// The admin console runs on a separate port so the target can be
		// repointed remotely.
		Web: WebConfig{Enabled: true, Addr: "0.0.0.0:8091"},
		Log: LogConfig{Level: "info"},
	}
}

// Load reads a YAML config from path, applies defaults to unset fields and then
// environment overrides. A missing file yields the default configuration.
func Load(path string) (*Config, error) {
	cfg := Default()
	if path != "" {
		data, err := os.ReadFile(path)
		switch {
		case err == nil:
			if err := yaml.Unmarshal(data, &cfg); err != nil {
				return nil, fmt.Errorf("parse config %s: %w", path, err)
			}
		case os.IsNotExist(err):
			// Keep defaults.
		default:
			return nil, fmt.Errorf("read config %s: %w", path, err)
		}
	}
	cfg.applyDefaults()
	cfg.applyEnv()
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (c *Config) applyDefaults() {
	d := Default()
	if c.Disguise.Scheme == "" {
		c.Disguise.Scheme = d.Disguise.Scheme
	}
	if c.Disguise.Host == "" {
		c.Disguise.Host = d.Disguise.Host
	}
	if c.Disguise.Port == 0 {
		c.Disguise.Port = d.Disguise.Port
	}
	if c.Disguise.TimeoutSeconds == 0 {
		c.Disguise.TimeoutSeconds = d.Disguise.TimeoutSeconds
	}
	if c.MCP.Transport == "" {
		c.MCP.Transport = d.MCP.Transport
	}
	if c.MCP.HTTP.Addr == "" {
		c.MCP.HTTP.Addr = d.MCP.HTTP.Addr
	}
	if c.Web.Addr == "" {
		c.Web.Addr = d.Web.Addr
	}
	if c.Log.Level == "" {
		c.Log.Level = d.Log.Level
	}
}

func (c *Config) applyEnv() {
	if v := os.Getenv("DISGUISEMCP_HOST"); v != "" {
		c.Disguise.Host = v
	}
	if v := os.Getenv("DISGUISEMCP_PORT"); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			c.Disguise.Port = p
		}
	}
	if v := os.Getenv("DISGUISEMCP_MCP_TRANSPORT"); v != "" {
		c.MCP.Transport = v
	}
	if v := os.Getenv("DISGUISEMCP_MCP_HTTP_ADDR"); v != "" {
		c.MCP.HTTP.Addr = v
	}
	if v := os.Getenv("DISGUISEMCP_WEB_ADDR"); v != "" {
		c.Web.Addr = v
	}
	if v := os.Getenv("DISGUISEMCP_WEB_ENABLED"); v != "" {
		c.Web.Enabled, _ = strconv.ParseBool(v)
	}
	if v := os.Getenv("DISGUISEMCP_LOG_LEVEL"); v != "" {
		c.Log.Level = v
	}
}

// Validate checks the configuration for internal consistency.
func (c *Config) Validate() error {
	switch c.MCP.Transport {
	case TransportStdio, TransportHTTP, TransportBoth:
	default:
		return fmt.Errorf("invalid mcp.transport %q (want stdio|http|both)", c.MCP.Transport)
	}
	if c.Disguise.Port < 0 || c.Disguise.Port > 65535 {
		return fmt.Errorf("invalid disguise.port %d", c.Disguise.Port)
	}
	switch c.Disguise.Scheme {
	case "http", "https":
	default:
		return fmt.Errorf("invalid disguise.scheme %q (want http|https)", c.Disguise.Scheme)
	}
	return nil
}

// Save writes the config to path as YAML using an atomic temp-file rename.
func (c *Config) Save(path string) error {
	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".disguise-mcp-*.yaml")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	return os.Rename(tmpName, path)
}

// Target returns the disguise client target for this config.
func (c *Config) Target() disguise.Target {
	return disguise.Target{Scheme: c.Disguise.Scheme, Host: c.Disguise.Host, Port: c.Disguise.Port}
}
