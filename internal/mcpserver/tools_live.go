package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/medcelerate/disguise-mcp/internal/disguise"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const liveURIPrefix = "disguise-live:///sub/"

func liveURI(id int) string { return liveURIPrefix + strconv.Itoa(id) }

func idFromURI(uri string) (int, bool) {
	if !strings.HasPrefix(uri, liveURIPrefix) {
		return 0, false
	}
	n, err := strconv.Atoi(strings.TrimPrefix(uri, liveURIPrefix))
	if err != nil {
		return 0, false
	}
	return n, true
}

// registerLiveTools registers the disguise Live Update tools and wires each
// subscription to an MCP resource with change notifications.
func registerLiveTools(s *mcp.Server, d *deps) {
	live := d.app.Live()

	// A static index resource listing all active live subscriptions. It also
	// ensures the resources capability is discoverable.
	s.AddResource(&mcp.Resource{
		URI:         "disguise-live:///subscriptions",
		Name:        "disguise live subscriptions",
		Description: "All active disguise Live Update subscriptions and their latest values.",
		MIMEType:    "application/json",
	}, func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		b, _ := json.Marshal(map[string]any{"subscriptions": d.app.Live().Get()})
		return &mcp.ReadResourceResult{Contents: []*mcp.ResourceContents{{
			URI: req.Params.URI, MIMEType: "application/json", Text: string(b),
		}}}, nil
	})

	// Push a resources/updated notification whenever a subscribed value changes.
	live.SetOnChange(func(v disguise.LiveValue) {
		_ = s.ResourceUpdated(context.Background(), &mcp.ResourceUpdatedNotificationParams{URI: liveURI(v.ID)})
	})

	addResource := func(v disguise.LiveValue) {
		s.AddResource(&mcp.Resource{
			URI:         liveURI(v.ID),
			Name:        fmt.Sprintf("live %s %s", v.Object, v.Property),
			Description: fmt.Sprintf("Live value of %q on %q (disguise Live Update).", v.Property, v.Object),
			MIMEType:    "application/json",
		}, func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
			id, ok := idFromURI(req.Params.URI)
			if !ok {
				return nil, fmt.Errorf("invalid live resource uri %q", req.Params.URI)
			}
			lv, found := d.app.Live().Value(id)
			if !found {
				return nil, fmt.Errorf("no live subscription %d", id)
			}
			b, _ := json.Marshal(lv)
			return &mcp.ReadResourceResult{Contents: []*mcp.ResourceContents{{
				URI: req.Params.URI, MIMEType: "application/json", Text: string(b),
			}}}, nil
		})
	}

	mcp.AddTool(s, &mcp.Tool{
		Name:        "disguise_live_subscribe",
		Description: "Subscribe to live updates for an object's properties over the disguise Live Update WebSocket. Object uses Designer expression syntax (e.g. \"track:track_1\"); properties use Python syntax (e.g. \"object.lengthInBeats\"). Each subscription is also exposed as an MCP resource that pushes change notifications.",
		Annotations: annWrite("Live subscribe"),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in liveSubscribeIn) (*mcp.CallToolResult, any, error) {
		if in.Object == "" || len(in.Properties) == 0 {
			return nil, nil, fmt.Errorf("object and properties are required")
		}
		vals, err := live.Subscribe(ctx, in.Object, in.Properties)
		if err != nil {
			return nil, nil, err
		}
		for _, v := range vals {
			addResource(v)
		}
		summary := fmt.Sprintf("Subscribed to %d property(ies) on %q.", len(vals), in.Object)
		return textResult(summary), map[string]any{"subscriptions": vals}, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "disguise_live_list",
		Description: "List all active disguise Live Update subscriptions and their latest cached values.",
		Annotations: annRead("List live subscriptions"),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, any, error) {
		vals := live.Get()
		return textResult(fmt.Sprintf("%d live subscription(s).", len(vals))), map[string]any{"subscriptions": vals}, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "disguise_live_get",
		Description: "Get the latest cached values for the given live-update subscription ids (empty returns all).",
		Annotations: annRead("Get live values"),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in liveIDsIn) (*mcp.CallToolResult, any, error) {
		vals := live.Get(in.IDs...)
		return textResult(fmt.Sprintf("%d value(s).", len(vals))), map[string]any{"subscriptions": vals}, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "disguise_live_set",
		Description: "Write property values live by subscription id (disguise Live Update 'set'). Changes are undoable and persistent on the Director.",
		Annotations: annDestructive("Live set property"),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in liveSetIn) (*mcp.CallToolResult, any, error) {
		if len(in.Sets) == 0 {
			return nil, nil, fmt.Errorf("at least one set item is required")
		}
		items := make([]disguise.SetItem, 0, len(in.Sets))
		for _, sv := range in.Sets {
			items = append(items, disguise.SetItem{ID: sv.ID, Value: sv.Value})
		}
		if err := live.Set(ctx, items); err != nil {
			return nil, nil, err
		}
		return textResult(fmt.Sprintf("Set %d value(s).", len(items))), map[string]any{"ok": true}, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "disguise_live_unsubscribe",
		Description: "Unsubscribe from live updates by subscription id and remove their MCP resources.",
		Annotations: annWrite("Live unsubscribe"),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in liveIDsIn) (*mcp.CallToolResult, any, error) {
		if len(in.IDs) == 0 {
			return nil, nil, fmt.Errorf("at least one id is required")
		}
		if err := live.Unsubscribe(ctx, in.IDs); err != nil {
			return nil, nil, err
		}
		uris := make([]string, 0, len(in.IDs))
		for _, id := range in.IDs {
			uris = append(uris, liveURI(id))
		}
		s.RemoveResources(uris...)
		return textResult(fmt.Sprintf("Unsubscribed %d.", len(in.IDs))), map[string]any{"ok": true}, nil
	})
}

type liveSubscribeIn struct {
	Object     string   `json:"object" jsonschema:"the object in Designer expression syntax, e.g. track:track_1"`
	Properties []string `json:"properties" jsonschema:"property paths in Python syntax, e.g. object.lengthInBeats"`
}

type liveIDsIn struct {
	IDs []int `json:"ids,omitempty" jsonschema:"subscription ids"`
}

type liveSetIn struct {
	Sets []liveSetOne `json:"sets" jsonschema:"the values to set, each by subscription id"`
}

type liveSetOne struct {
	ID    int `json:"id" jsonschema:"subscription id"`
	Value any `json:"value" jsonschema:"the value to set"`
}
