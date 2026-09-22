package disguise

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coder/websocket"
)

type recorder struct {
	mu    sync.Mutex
	sets  []string
	unsub []string
}

func fakeLive(t *testing.T) (Target, *recorder, func()) {
	t.Helper()
	rec := &recorder{}
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
		if err != nil {
			return
		}
		defer func() { _ = c.Close(websocket.StatusNormalClosure, "") }()
		ctx := context.Background()
		for {
			_, data, err := c.Read(ctx)
			if err != nil {
				return
			}
			var m map[string]json.RawMessage
			if err := json.Unmarshal(data, &m); err != nil {
				continue
			}
			if sub, ok := m["subscribe"]; ok {
				var body struct {
					Object     string   `json:"object"`
					Properties []string `json:"properties"`
				}
				_ = json.Unmarshal(sub, &body)
				var entries []map[string]any
				for i, p := range body.Properties {
					entries = append(entries, map[string]any{"id": 100 + i, "objectPath": body.Object, "propertyPath": p})
				}
				resp, _ := json.Marshal(map[string]any{"subscriptions": entries})
				_ = c.Write(ctx, websocket.MessageText, resp)
				vc, _ := json.Marshal(map[string]any{"valuesChanged": []map[string]any{
					{"id": 100, "value": 42, "changeTimestamp": 1.0, "messageTimestamp": 2.0},
				}})
				_ = c.Write(ctx, websocket.MessageText, vc)
			}
			if s, ok := m["set"]; ok {
				rec.mu.Lock()
				rec.sets = append(rec.sets, string(s))
				rec.mu.Unlock()
			}
			if u, ok := m["unsubscribe"]; ok {
				rec.mu.Lock()
				rec.unsub = append(rec.unsub, string(u))
				rec.mu.Unlock()
			}
		}
	})
	srv := httptest.NewServer(h)
	u, _ := url.Parse(srv.URL)
	port, _ := strconv.Atoi(u.Port())
	return Target{Scheme: "http", Host: u.Hostname(), Port: port}, rec, srv.Close
}

func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("condition not met within timeout")
}

func TestLiveSubscribeValuesSetUnsubscribe(t *testing.T) {
	target, rec, closeSrv := fakeLive(t)
	defer closeSrv()

	var changed atomic.Int32
	lc := NewLiveClient(func() Target { return target }, nil)
	lc.SetOnChange(func(LiveValue) { changed.Add(1) })
	defer lc.Reset()

	ctx := context.Background()
	vals, err := lc.Subscribe(ctx, "track:t1", []string{"object.lengthInBeats", "object.layers"})
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	if len(vals) != 2 || vals[0].ID != 100 || vals[1].ID != 101 {
		t.Fatalf("subscriptions = %+v", vals)
	}
	if vals[0].Property != "object.lengthInBeats" {
		t.Fatalf("property = %q", vals[0].Property)
	}

	// The pushed value for id 100 should arrive and update the cache.
	waitFor(t, func() bool {
		lv, ok := lc.Value(100)
		return ok && lv.Updated && string(lv.Value) == "42"
	})
	if changed.Load() == 0 {
		t.Fatal("onChange not called")
	}

	// Set a value; the server should receive it.
	if err := lc.Set(ctx, []SetItem{{ID: 100, Value: 7}}); err != nil {
		t.Fatalf("set: %v", err)
	}
	waitFor(t, func() bool {
		rec.mu.Lock()
		defer rec.mu.Unlock()
		return len(rec.sets) == 1 && jsonHas(rec.sets[0], `"id":100`) && jsonHas(rec.sets[0], `"value":7`)
	})

	// Unsubscribe removes the cached subscription.
	if err := lc.Unsubscribe(ctx, []int{100, 101}); err != nil {
		t.Fatalf("unsubscribe: %v", err)
	}
	if _, ok := lc.Value(100); ok {
		t.Fatal("value 100 still present after unsubscribe")
	}
	waitFor(t, func() bool {
		rec.mu.Lock()
		defer rec.mu.Unlock()
		return len(rec.unsub) == 1 && jsonHas(rec.unsub[0], `"ids":[100,101]`)
	})
}

func jsonHas(s, sub string) bool {
	// compact both to ignore whitespace differences
	var v any
	if json.Unmarshal([]byte(s), &v) == nil {
		b, _ := json.Marshal(v)
		s = string(b)
	}
	return contains(s, sub)
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
