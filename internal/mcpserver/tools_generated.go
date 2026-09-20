package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/medcelerate/disguise-mcp/internal/disguise"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// registerGeneratedTools registers one MCP tool per disguise API operation from
// the embedded OpenAPI specs. When enabledTags is non-empty, only operations in
// those sections (tags, lower-cased) are registered. Returns the number added.
func registerGeneratedTools(s *mcp.Server, d *deps, ops []disguise.Operation, enabledTags map[string]bool) int {
	count := 0
	for _, op := range ops {
		if len(enabledTags) > 0 && !enabledTags[strings.ToLower(op.Tag)] {
			continue
		}
		ann := &mcp.ToolAnnotations{Title: title(op)}
		if op.ReadOnly() {
			ann.ReadOnlyHint = true
		} else {
			ann.DestructiveHint = boolPtr(op.Destructive())
		}
		tool := &mcp.Tool{
			Name:        op.ToolName(),
			Description: description(op),
			InputSchema: op.InputSchema(),
			Annotations: ann,
		}
		s.AddTool(tool, d.makeHandler(op))
		count++
	}
	return count
}

// makeHandler builds the low-level handler that invokes a disguise endpoint.
func (d *deps) makeHandler(op disguise.Operation) mcp.ToolHandler {
	return func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := map[string]any{}
		if len(req.Params.Arguments) > 0 {
			if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
				return nil, fmt.Errorf("invalid arguments: %w", err)
			}
		}
		path := op.Path
		if q := buildQuery(op, args); q != "" {
			path += "?" + q
		}
		var body any
		if b, ok := args["body"]; ok {
			body = b
		}
		raw, err := d.app.Client().Do(ctx, op.Method, path, body)
		if err != nil {
			return nil, err
		}
		res, _, _ := jsonResult(op.ID+":", raw)
		return res, nil
	}
}

func buildQuery(op disguise.Operation, args map[string]any) string {
	q := url.Values{}
	for _, p := range op.Query {
		v, ok := args[p.Name]
		if !ok || v == nil {
			continue
		}
		q.Set(p.Name, fmt.Sprint(v))
	}
	return q.Encode()
}

// title humanizes an operationId, e.g. "Transport_Play" -> "Transport: Play".
func title(op disguise.Operation) string {
	if i := strings.Index(op.ID, "_"); i >= 0 {
		return op.ID[:i] + ": " + op.ID[i+1:]
	}
	return op.ID
}

// description prefers the spec summary, falling back to method+path, and always
// notes the section.
func description(op disguise.Operation) string {
	desc := strings.TrimSpace(op.Summary)
	if extra := strings.TrimSpace(op.Description); extra != "" && extra != desc {
		if desc != "" {
			desc += " — " + extra
		} else {
			desc = extra
		}
	}
	if desc == "" {
		desc = fmt.Sprintf("%s %s", op.Method, op.Path)
	}
	if op.Tag != "" {
		desc += fmt.Sprintf(" [%s]", op.Tag)
	}
	return desc
}
