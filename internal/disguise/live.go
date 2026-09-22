package disguise

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/coder/websocket"
)

// The disguise Live Update API is a WebSocket at /api/session/liveupdate on the
// Designer API host. Clients subscribe to an object's properties and receive
// pushed value changes, and can write values back.
//
// Protocol (JSON messages):
//   subscribe:    {"subscribe": {"object": "...", "properties": ["...", ...]}}
//   subscribed:   {"subscriptions": [{"id": N, "objectPath": "...", "propertyPath": "..."}]}
//   valuesChanged:{"valuesChanged": [{"id": N, "value": <v>, "changeTimestamp": f, "messageTimestamp": f}]}
//   set:          {"set": [{"id": N, "value": <v>}]}
//   unsubscribe:  {"unsubscribe": {"ids": [N, ...]}}
//   error:        {"error": <message>}

// LiveValue is a cached live-update subscription and its latest value.
type LiveValue struct {
	ID               int             `json:"id"`
	Object           string          `json:"object,omitempty"`
	Property         string          `json:"property,omitempty"`
	Value            json.RawMessage `json:"value,omitempty"`
	ChangeTimestamp  float64         `json:"changeTimestamp,omitempty"`
	MessageTimestamp float64         `json:"messageTimestamp,omitempty"`
	Updated          bool            `json:"-"`
}

// SetItem is one property write.
type SetItem struct {
	ID    int `json:"id"`
	Value any `json:"value"`
}

// wire types
type subEntry struct {
	ID           int    `json:"id"`
	ObjectPath   string `json:"objectPath"`
	PropertyPath string `json:"propertyPath"`
}

type valueChange struct {
	ID               int             `json:"id"`
	Value            json.RawMessage `json:"value"`
	ChangeTimestamp  float64         `json:"changeTimestamp"`
	MessageTimestamp float64         `json:"messageTimestamp"`
}

type inbound struct {
	Subscriptions []subEntry      `json:"subscriptions"`
	ValuesChanged []valueChange   `json:"valuesChanged"`
	Error         json.RawMessage `json:"error"`
}

// LiveClient manages a WebSocket connection to the disguise Live Update API,
// caches subscription values, and pushes change notifications to a callback.
type LiveClient struct {
	targetFn func() Target
	logf     func(string, ...any)

	mu       sync.Mutex
	conn     *websocket.Conn
	subs     map[int]*LiveValue
	onChange func(LiveValue)

	writeMu sync.Mutex
	subMu   sync.Mutex      // serialises subscribe request/response
	subCh   chan []subEntry // read loop delivers subscription responses here
}

// NewLiveClient creates a live client. targetFn supplies the current disguise
// target; logf may be nil.
func NewLiveClient(targetFn func() Target, logf func(string, ...any)) *LiveClient {
	if logf == nil {
		logf = func(string, ...any) {}
	}
	return &LiveClient{
		targetFn: targetFn,
		logf:     logf,
		subs:     make(map[int]*LiveValue),
		subCh:    make(chan []subEntry, 1),
	}
}

// SetOnChange registers a callback invoked whenever a subscribed value changes.
func (c *LiveClient) SetOnChange(fn func(LiveValue)) {
	c.mu.Lock()
	c.onChange = fn
	c.mu.Unlock()
}

func (c *LiveClient) wsURL() string {
	t := c.targetFn().withDefaults()
	scheme := "ws"
	if t.Scheme == "https" {
		scheme = "wss"
	}
	return fmt.Sprintf("%s://%s:%d/api/session/liveupdate", scheme, t.Host, t.Port)
}

// ensureConn dials the WebSocket if not already connected.
func (c *LiveClient) ensureConn(ctx context.Context) (*websocket.Conn, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn != nil {
		return c.conn, nil
	}
	url := c.wsURL()
	dialCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	conn, _, err := websocket.Dial(dialCtx, url, nil)
	if err != nil {
		return nil, fmt.Errorf("connect live update %s: %w", url, err)
	}
	conn.SetReadLimit(16 << 20)
	c.conn = conn
	go c.readLoop(conn)
	c.logf("live update connected to %s", url)
	return conn, nil
}

func (c *LiveClient) readLoop(conn *websocket.Conn) {
	ctx := context.Background()
	for {
		_, data, err := conn.Read(ctx)
		if err != nil {
			c.mu.Lock()
			if c.conn == conn {
				c.conn = nil
			}
			c.mu.Unlock()
			return
		}
		var msg inbound
		if err := json.Unmarshal(data, &msg); err != nil {
			continue
		}
		if len(msg.Subscriptions) > 0 {
			select {
			case c.subCh <- msg.Subscriptions:
			default:
			}
		}
		for _, vc := range msg.ValuesChanged {
			c.applyChange(vc)
		}
		if len(msg.Error) > 0 {
			c.logf("live update error: %s", string(msg.Error))
		}
	}
}

func (c *LiveClient) applyChange(vc valueChange) {
	c.mu.Lock()
	lv, ok := c.subs[vc.ID]
	if !ok {
		lv = &LiveValue{ID: vc.ID}
		c.subs[vc.ID] = lv
	}
	lv.Value = vc.Value
	lv.ChangeTimestamp = vc.ChangeTimestamp
	lv.MessageTimestamp = vc.MessageTimestamp
	lv.Updated = true
	snapshot := *lv
	onChange := c.onChange
	c.mu.Unlock()
	if onChange != nil {
		onChange(snapshot)
	}
}

func (c *LiveClient) write(ctx context.Context, v any) error {
	conn, err := c.ensureConn(ctx)
	if err != nil {
		return err
	}
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	return conn.Write(ctx, websocket.MessageText, data)
}

// Subscribe subscribes to an object's properties and returns the created
// subscriptions (values populate asynchronously as changes arrive).
func (c *LiveClient) Subscribe(ctx context.Context, object string, properties []string) ([]LiveValue, error) {
	if object == "" || len(properties) == 0 {
		return nil, fmt.Errorf("object and at least one property are required")
	}
	c.subMu.Lock()
	defer c.subMu.Unlock()

	// Drain any stale response.
	select {
	case <-c.subCh:
	default:
	}

	req := map[string]any{"subscribe": map[string]any{"object": object, "properties": properties}}
	if err := c.write(ctx, req); err != nil {
		return nil, err
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(10 * time.Second):
		return nil, fmt.Errorf("timed out awaiting subscription response")
	case entries := <-c.subCh:
		out := make([]LiveValue, 0, len(entries))
		c.mu.Lock()
		for _, e := range entries {
			lv, ok := c.subs[e.ID]
			if !ok {
				lv = &LiveValue{ID: e.ID}
				c.subs[e.ID] = lv
			}
			lv.Object = e.ObjectPath
			lv.Property = e.PropertyPath
			out = append(out, *lv)
		}
		c.mu.Unlock()
		return out, nil
	}
}

// Get returns the cached values for the given ids (all if ids is empty).
func (c *LiveClient) Get(ids ...int) []LiveValue {
	c.mu.Lock()
	defer c.mu.Unlock()
	var out []LiveValue
	if len(ids) == 0 {
		for _, lv := range c.subs {
			out = append(out, *lv)
		}
		return out
	}
	for _, id := range ids {
		if lv, ok := c.subs[id]; ok {
			out = append(out, *lv)
		}
	}
	return out
}

// Value returns the cached value for one id.
func (c *LiveClient) Value(id int) (LiveValue, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if lv, ok := c.subs[id]; ok {
		return *lv, true
	}
	return LiveValue{}, false
}

// Set writes one or more property values by subscription id.
func (c *LiveClient) Set(ctx context.Context, items []SetItem) error {
	if len(items) == 0 {
		return fmt.Errorf("at least one set item is required")
	}
	return c.write(ctx, map[string]any{"set": items})
}

// Unsubscribe removes subscriptions by id.
func (c *LiveClient) Unsubscribe(ctx context.Context, ids []int) error {
	if len(ids) == 0 {
		return fmt.Errorf("at least one id is required")
	}
	if err := c.write(ctx, map[string]any{"unsubscribe": map[string]any{"ids": ids}}); err != nil {
		return err
	}
	c.mu.Lock()
	for _, id := range ids {
		delete(c.subs, id)
	}
	c.mu.Unlock()
	return nil
}

// Reset drops the connection and clears subscriptions (used when the target is
// repointed at a different disguise server).
func (c *LiveClient) Reset() {
	c.mu.Lock()
	conn := c.conn
	c.conn = nil
	c.subs = make(map[int]*LiveValue)
	c.mu.Unlock()
	if conn != nil {
		_ = conn.Close(websocket.StatusNormalClosure, "reset")
	}
}
