package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/medcelerate/disguise-mcp/internal/disguise"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// registerControlTools registers connection/status and generic API tools.
func registerControlTools(s *mcp.Server, d *deps) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "disguise_status",
		Description: "Report which disguise server the bridge is pointed at and whether it is reachable.",
		Annotations: annRead("disguise status"),
	}, d.status)

	mcp.AddTool(s, &mcp.Tool{
		Name:        "disguise_set_target",
		Description: "Repoint the bridge at a different disguise server by host and port. Persists to the config file. The disguise Designer API defaults to port 80.",
		Annotations: annWrite("Set disguise target"),
	}, d.setTarget)

	mcp.AddTool(s, &mcp.Tool{
		Name:        "disguise_system",
		Description: "Get system information from the disguise server (Service API /api/service/system).",
		Annotations: annRead("System info"),
	}, d.system)

	mcp.AddTool(s, &mcp.Tool{
		Name:        "disguise_project",
		Description: "Get project information from the disguise server (Service API /api/service/project).",
		Annotations: annRead("Project info"),
	}, d.project)

	mcp.AddTool(s, &mcp.Tool{
		Name:        "disguise_raw",
		Description: "Make an arbitrary disguise Designer API request. Provide the HTTP method, the API path (e.g. /api/service/tasks), and an optional JSON body. Use for endpoints without a dedicated tool.",
		Annotations: annDestructive("Raw disguise API call"),
	}, d.raw)
}

// --- disguise_status ---

func (d *deps) status(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, any, error) {
	st := d.app.Status(ctx)
	summary := fmt.Sprintf("disguise target %s — reachable=%v", st.BaseURL, st.Reachable)
	if st.Error != "" {
		summary += " (" + st.Error + ")"
	}
	res := &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: summary}}}
	return res, st, nil
}

// --- disguise_set_target ---

type setTargetIn struct {
	Host   string `json:"host" jsonschema:"hostname or IP of the disguise server"`
	Port   int    `json:"port,omitempty" jsonschema:"API port (default 80)"`
	Scheme string `json:"scheme,omitempty" jsonschema:"http or https (default http)"`
}

func (d *deps) setTarget(ctx context.Context, _ *mcp.CallToolRequest, in setTargetIn) (*mcp.CallToolResult, any, error) {
	if in.Host == "" {
		return nil, nil, fmt.Errorf("host is required")
	}
	if err := d.app.SetTarget(disguise.Target{Scheme: in.Scheme, Host: in.Host, Port: in.Port}); err != nil {
		return nil, nil, err
	}
	st := d.app.Status(ctx)
	summary := fmt.Sprintf("Target set to %s — reachable=%v", st.BaseURL, st.Reachable)
	if st.Error != "" {
		summary += " (" + st.Error + ")"
	}
	res := &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: summary}}}
	return res, st, nil
}

// --- service reads ---

func (d *deps) system(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, any, error) {
	raw, err := d.app.Client().Get(ctx, "/api/service/system")
	if err != nil {
		return nil, nil, err
	}
	return jsonResult("System info:", raw)
}

func (d *deps) project(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, any, error) {
	raw, err := d.app.Client().Get(ctx, "/api/service/project")
	if err != nil {
		return nil, nil, err
	}
	return jsonResult("Project info:", raw)
}

// --- disguise_raw ---

type rawIn struct {
	Method string `json:"method,omitempty" jsonschema:"HTTP method: GET or POST (default GET)"`
	Path   string `json:"path" jsonschema:"API path, e.g. /api/service/tasks"`
	Body   string `json:"body,omitempty" jsonschema:"optional JSON request body"`
}

func (d *deps) raw(ctx context.Context, _ *mcp.CallToolRequest, in rawIn) (*mcp.CallToolResult, any, error) {
	if in.Path == "" {
		return nil, nil, fmt.Errorf("path is required")
	}
	method := strings.ToUpper(in.Method)
	if method == "" {
		method = "GET"
	}
	var body any
	if strings.TrimSpace(in.Body) != "" {
		if err := json.Unmarshal([]byte(in.Body), &body); err != nil {
			return nil, nil, fmt.Errorf("body is not valid JSON: %w", err)
		}
	}
	raw, err := d.app.Client().Do(ctx, method, in.Path, body)
	if err != nil {
		return nil, nil, err
	}
	return jsonResult(fmt.Sprintf("%s %s:", method, in.Path), raw)
}
