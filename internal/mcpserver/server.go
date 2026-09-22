// Package mcpserver builds the Model Context Protocol server that exposes the
// disguise Designer API as tools. Most tools are generated directly from the
// embedded OpenAPI specs so coverage tracks the API; a few control tools
// (status, set-target, raw) are hand-written.
package mcpserver

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/medcelerate/disguise-mcp/internal/app"
	"github.com/medcelerate/disguise-mcp/internal/config"
	"github.com/medcelerate/disguise-mcp/internal/disguise"
	"github.com/medcelerate/disguise-mcp/internal/version"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// deps carries shared dependencies for tool handlers.
type deps struct {
	app *app.App
}

// New builds an MCP server with control tools plus a tool per disguise API
// operation. cfg.Disguise.Sections, if set, restricts which API sections
// (tags) are exposed. Returns the server and the number of generated tools.
func New(a *app.App, cfg *config.Config) (*mcp.Server, int, error) {
	s := mcp.NewServer(&mcp.Implementation{
		Name:    "disguise-mcp",
		Version: version.Version,
	}, &mcp.ServerOptions{
		// Advertise the resources capability (with subscribe support) so clients
		// can subscribe to live-update resources for pushed change notifications.
		// Tools/prompts capabilities are still inferred from what's registered.
		Capabilities: &mcp.ServerCapabilities{
			Resources: &mcp.ResourceCapabilities{Subscribe: true, ListChanged: true},
		},
		SubscribeHandler:   func(context.Context, *mcp.SubscribeRequest) error { return nil },
		UnsubscribeHandler: func(context.Context, *mcp.UnsubscribeRequest) error { return nil },
	})

	d := &deps{app: a}
	registerControlTools(s, d)
	registerLiveTools(s, d)

	ops, err := disguise.LoadOperations()
	if err != nil {
		return nil, 0, err
	}
	enabled := map[string]bool{}
	for _, t := range cfg.Disguise.Sections {
		enabled[strings.ToLower(t)] = true
	}
	n := registerGeneratedTools(s, d, ops, enabled)
	return s, n, nil
}

// jsonResult builds a tool result: a text summary plus the raw disguise JSON,
// and the decoded JSON as structured content.
func jsonResult(summary string, raw json.RawMessage) (*mcp.CallToolResult, any, error) {
	text := summary
	if len(raw) > 0 && string(raw) != "null" {
		text = summary + "\n" + string(raw)
	}
	res := &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: text}}}
	var v any
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &v)
	}
	return res, v, nil
}

// textResult wraps a human-readable summary as tool content.
func textResult(summary string) *mcp.CallToolResult {
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: summary}}}
}

func boolPtr(b bool) *bool { return &b }

func annRead(title string) *mcp.ToolAnnotations {
	return &mcp.ToolAnnotations{Title: title, ReadOnlyHint: true}
}
func annWrite(title string) *mcp.ToolAnnotations {
	return &mcp.ToolAnnotations{Title: title, DestructiveHint: boolPtr(false)}
}
func annDestructive(title string) *mcp.ToolAnnotations {
	return &mcp.ToolAnnotations{Title: title, DestructiveHint: boolPtr(true)}
}
