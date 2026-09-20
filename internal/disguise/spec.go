package disguise

import (
	"embed"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

//go:embed specs/service.swagger.json specs/session.swagger.json
var specFS embed.FS

// Param is a query parameter of an operation.
type Param struct {
	Name        string
	Required    bool
	Description string
	Schema      json.RawMessage // JSON Schema for this parameter's value
}

// Operation is one disguise API endpoint distilled from the OpenAPI specs.
type Operation struct {
	ID           string // operationId, e.g. "Transport_Play"
	Tag          string // section, e.g. "Transport"
	Method       string // GET, POST, ...
	Path         string // full path including basePath, e.g. /api/session/transport/play
	Summary      string
	Description  string
	Query        []Param
	Body         json.RawMessage // resolved request body schema, nil if none
	BodyRequired bool
}

var destructiveRe = regexp.MustCompile(`(?i)(remove|delete|quit|failover|restart|clear|reset|revert|discard|stop|abort)`)

// ReadOnly reports whether the operation only reads state.
func (o Operation) ReadOnly() bool { return o.Method == "GET" }

// Destructive reports whether the operation may remove or overwrite state.
func (o Operation) Destructive() bool {
	if o.Method == "DELETE" {
		return true
	}
	return o.Method != "GET" && destructiveRe.MatchString(o.ID)
}

var nonName = regexp.MustCompile(`[^a-z0-9]+`)

// ToolName returns the MCP tool name for this operation, e.g.
// "disguise_transport_play". Names are <= 64 chars.
func (o Operation) ToolName() string {
	n := "disguise_" + nonName.ReplaceAllString(strings.ToLower(o.ID), "_")
	n = strings.Trim(n, "_")
	if len(n) > 64 {
		n = n[:64]
	}
	return n
}

// InputSchema builds the JSON Schema (object) for the tool's arguments: one
// property per query parameter, plus a "body" property when the operation takes
// a request body.
func (o Operation) InputSchema() json.RawMessage {
	props := map[string]any{}
	var required []string
	for _, p := range o.Query {
		var sch any
		if len(p.Schema) > 0 {
			_ = json.Unmarshal(p.Schema, &sch)
		} else {
			sch = map[string]any{"type": "string"}
		}
		props[p.Name] = sch
		if p.Required {
			required = append(required, p.Name)
		}
	}
	if o.Body != nil {
		var b any
		_ = json.Unmarshal(o.Body, &b)
		props["body"] = b
		if o.BodyRequired {
			required = append(required, "body")
		}
	}
	schema := map[string]any{"type": "object", "properties": props}
	if len(required) > 0 {
		sort.Strings(required)
		schema["required"] = required
	}
	out, _ := json.Marshal(schema)
	return out
}

// --- swagger parsing ---

type swaggerDoc struct {
	BasePath    string                                `json:"basePath"`
	Paths       map[string]map[string]json.RawMessage `json:"paths"`
	Definitions map[string]json.RawMessage            `json:"definitions"`
}

type swaggerOp struct {
	OperationID string         `json:"operationId"`
	Tags        []string       `json:"tags"`
	Summary     string         `json:"summary"`
	Description string         `json:"description"`
	Parameters  []swaggerParam `json:"parameters"`
}

type swaggerParam struct {
	Name        string          `json:"name"`
	In          string          `json:"in"`
	Required    bool            `json:"required"`
	Description string          `json:"description"`
	Type        string          `json:"type"`
	Format      string          `json:"format"`
	Enum        []any           `json:"enum"`
	Items       json.RawMessage `json:"items"`
	Schema      json.RawMessage `json:"schema"`
}

var httpMethods = map[string]bool{"get": true, "post": true, "put": true, "delete": true, "patch": true}

// LoadOperations parses the embedded specs and returns all operations, sorted
// by tag then id.
func LoadOperations() ([]Operation, error) {
	var ops []Operation
	for _, name := range []string{"specs/service.swagger.json", "specs/session.swagger.json"} {
		data, err := specFS.ReadFile(name)
		if err != nil {
			return nil, err
		}
		var doc swaggerDoc
		if err := json.Unmarshal(data, &doc); err != nil {
			return nil, fmt.Errorf("parse %s: %w", name, err)
		}
		for path, methods := range doc.Paths {
			for method, rawOp := range methods {
				if !httpMethods[strings.ToLower(method)] {
					continue
				}
				var so swaggerOp
				if err := json.Unmarshal(rawOp, &so); err != nil {
					return nil, fmt.Errorf("parse op %s %s: %w", method, path, err)
				}
				op := Operation{
					ID:          so.OperationID,
					Method:      strings.ToUpper(method),
					Path:        doc.BasePath + path,
					Summary:     so.Summary,
					Description: so.Description,
				}
				if len(so.Tags) > 0 {
					op.Tag = so.Tags[0]
				}
				for _, p := range so.Parameters {
					switch p.In {
					case "body":
						op.Body = resolveJSON(p.Schema, doc.Definitions, map[string]bool{})
						op.BodyRequired = p.Required
					case "query":
						op.Query = append(op.Query, Param{
							Name:        p.Name,
							Required:    p.Required,
							Description: p.Description,
							Schema:      paramSchema(p),
						})
					}
				}
				if op.ID == "" {
					continue
				}
				ops = append(ops, op)
			}
		}
	}
	sort.Slice(ops, func(i, j int) bool {
		if ops[i].Tag != ops[j].Tag {
			return ops[i].Tag < ops[j].Tag
		}
		return ops[i].ID < ops[j].ID
	})
	return ops, nil
}

// paramSchema builds a JSON Schema for a non-body swagger parameter.
func paramSchema(p swaggerParam) json.RawMessage {
	m := map[string]any{}
	if p.Type != "" {
		m["type"] = p.Type
	}
	if p.Format != "" {
		m["format"] = p.Format
	}
	if p.Description != "" {
		m["description"] = p.Description
	}
	if len(p.Enum) > 0 {
		m["enum"] = p.Enum
	}
	if len(p.Items) > 0 {
		var items any
		_ = json.Unmarshal(p.Items, &items)
		m["items"] = items
	}
	out, _ := json.Marshal(m)
	return out
}

// resolveJSON inlines $ref references against defs, guarding against cycles.
func resolveJSON(raw json.RawMessage, defs map[string]json.RawMessage, seen map[string]bool) json.RawMessage {
	var node any
	if err := json.Unmarshal(raw, &node); err != nil {
		return raw
	}
	resolved := resolveNode(node, defs, seen)
	out, _ := json.Marshal(resolved)
	return out
}

func resolveNode(node any, defs map[string]json.RawMessage, seen map[string]bool) any {
	switch v := node.(type) {
	case map[string]any:
		if ref, ok := v["$ref"].(string); ok {
			name := strings.TrimPrefix(ref, "#/definitions/")
			if seen[name] {
				return map[string]any{"type": "object", "description": "(recursive " + name + ")"}
			}
			def, ok := defs[name]
			if !ok {
				return map[string]any{"type": "object"}
			}
			next := cloneSeen(seen)
			next[name] = true
			var dnode any
			_ = json.Unmarshal(def, &dnode)
			return resolveNode(dnode, defs, next)
		}
		out := make(map[string]any, len(v))
		for k, val := range v {
			out[k] = resolveNode(val, defs, seen)
		}
		return out
	case []any:
		for i := range v {
			v[i] = resolveNode(v[i], defs, seen)
		}
		return v
	default:
		return node
	}
}

func cloneSeen(s map[string]bool) map[string]bool {
	out := make(map[string]bool, len(s)+1)
	for k := range s {
		out[k] = true
	}
	return out
}
