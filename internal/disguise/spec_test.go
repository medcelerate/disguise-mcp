package disguise

import (
	"encoding/json"
	"testing"
)

func TestLoadOperations(t *testing.T) {
	ops, err := LoadOperations()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	// Service (13) + Session (74) endpoints.
	if len(ops) < 80 {
		t.Fatalf("expected 80+ operations, got %d", len(ops))
	}

	byID := map[string]Operation{}
	for _, o := range ops {
		byID[o.ID] = o
	}

	play, ok := byID["Transport_Play"]
	if !ok {
		t.Fatal("Transport_Play not found")
	}
	if play.Method != "POST" || play.Path != "/api/session/transport/play" {
		t.Fatalf("Transport_Play = %s %s", play.Method, play.Path)
	}
	if play.Body == nil {
		t.Fatal("Transport_Play should have a resolved body schema")
	}
	if play.ToolName() != "disguise_transport_play" {
		t.Fatalf("tool name = %q", play.ToolName())
	}
	if play.ReadOnly() {
		t.Fatal("Transport_Play is not read-only")
	}

	osinfo, ok := byID["System_GetOsInfo"]
	if !ok {
		t.Fatal("System_GetOsInfo not found")
	}
	if osinfo.Method != "GET" || !osinfo.ReadOnly() {
		t.Fatalf("System_GetOsInfo should be a read-only GET, got %s readonly=%v", osinfo.Method, osinfo.ReadOnly())
	}

	remove, ok := byID["Media_Remove"]
	if !ok {
		t.Fatal("Media_Remove not found")
	}
	if !remove.Destructive() {
		t.Fatal("Media_Remove should be marked destructive")
	}
}

func TestInputSchemaIsObject(t *testing.T) {
	ops, err := LoadOperations()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	for _, o := range ops {
		var m map[string]any
		if err := json.Unmarshal(o.InputSchema(), &m); err != nil {
			t.Fatalf("%s: invalid input schema: %v", o.ID, err)
		}
		if m["type"] != "object" {
			t.Fatalf("%s: input schema type = %v, want object", o.ID, m["type"])
		}
	}
}
