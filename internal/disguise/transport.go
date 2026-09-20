package disguise

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
)

// Ref identifies a transport, track or section by uid and/or name. disguise
// matches on either; sending just a name is usually sufficient.
type Ref struct {
	UID  string `json:"uid,omitempty"`
	Name string `json:"name,omitempty"`
}

const transportBase = "/api/session/transport"

// --- Reads ---

// Transports returns all transports and multitransports.
func (c *Client) Transports(ctx context.Context) (json.RawMessage, error) {
	return c.Get(ctx, transportBase+"/transports")
}

// ActiveTransport returns the currently active transport(s) and their state.
func (c *Client) ActiveTransport(ctx context.Context) (json.RawMessage, error) {
	return c.Get(ctx, transportBase+"/activetransport")
}

// Tracks returns the list of tracks.
func (c *Client) Tracks(ctx context.Context) (json.RawMessage, error) {
	return c.Get(ctx, transportBase+"/tracks")
}

// Setlists returns the list of setlists.
func (c *Client) Setlists(ctx context.Context) (json.RawMessage, error) {
	return c.Get(ctx, transportBase+"/setlists")
}

// Annotations returns notes, tags and sections for a transport.
func (c *Client) Annotations(ctx context.Context, ref Ref) (json.RawMessage, error) {
	q := url.Values{}
	if ref.UID != "" {
		q.Set("uid", ref.UID)
	}
	if ref.Name != "" {
		q.Set("name", ref.Name)
	}
	return c.Get(ctx, transportBase+"/annotations?"+q.Encode())
}

// --- Simple actions (transports: [{uid,name}]) ---

func (c *Client) transportsAction(ctx context.Context, action string, refs []Ref) (json.RawMessage, error) {
	if len(refs) == 0 {
		return nil, fmt.Errorf("at least one transport is required")
	}
	return c.Post(ctx, transportBase+"/"+action, map[string]any{"transports": refs})
}

// Play sets the given transports to play.
func (c *Client) Play(ctx context.Context, refs []Ref) (json.RawMessage, error) {
	return c.transportsAction(ctx, "play", refs)
}

// Stop stops the given transports.
func (c *Client) Stop(ctx context.Context, refs []Ref) (json.RawMessage, error) {
	return c.transportsAction(ctx, "stop", refs)
}

// ReturnToStart returns the given transports to the start.
func (c *Client) ReturnToStart(ctx context.Context, refs []Ref) (json.RawMessage, error) {
	return c.transportsAction(ctx, "returntostart", refs)
}

// PlaySection plays to the end of the current section.
func (c *Client) PlaySection(ctx context.Context, refs []Ref) (json.RawMessage, error) {
	return c.transportsAction(ctx, "playsection", refs)
}

// PlayLoopSection loops the current section.
func (c *Client) PlayLoopSection(ctx context.Context, refs []Ref) (json.RawMessage, error) {
	return c.transportsAction(ctx, "playloopsection", refs)
}

// --- Navigation (transports: [{transport:{uid,name}, ...}]) ---

// navItem wraps a single transport plus per-action fields.
func navPost(ctx context.Context, c *Client, action string, item map[string]any) (json.RawMessage, error) {
	return c.Post(ctx, transportBase+"/"+action, map[string]any{"transports": []map[string]any{item}})
}

// GotoTime seeks to a time (seconds). playmode may be "" for default.
func (c *Client) GotoTime(ctx context.Context, ref Ref, time float64, playmode string) (json.RawMessage, error) {
	return navPost(ctx, c, "gototime", map[string]any{"transport": ref, "time": time, "playmode": playmode})
}

// GotoTimecode seeks to a timecode string, e.g. "01:00:00:00".
func (c *Client) GotoTimecode(ctx context.Context, ref Ref, timecode string, ignoreTags bool, playmode string) (json.RawMessage, error) {
	return navPost(ctx, c, "gototimecode", map[string]any{"transport": ref, "timecode": timecode, "ignoreTags": ignoreTags, "playmode": playmode})
}

// GotoFrame seeks to a frame number.
func (c *Client) GotoFrame(ctx context.Context, ref Ref, frame int, playmode string) (json.RawMessage, error) {
	return navPost(ctx, c, "gotoframe", map[string]any{"transport": ref, "frame": frame, "playmode": playmode})
}

// GotoSection jumps to a named section.
func (c *Client) GotoSection(ctx context.Context, ref Ref, section, playmode string) (json.RawMessage, error) {
	return navPost(ctx, c, "gotosection", map[string]any{"transport": ref, "section": section, "playmode": playmode})
}

// GotoNextSection / GotoPrevSection step between sections.
func (c *Client) GotoNextSection(ctx context.Context, ref Ref, playmode string) (json.RawMessage, error) {
	return navPost(ctx, c, "gotonextsection", map[string]any{"transport": ref, "playmode": playmode})
}
func (c *Client) GotoPrevSection(ctx context.Context, ref Ref, playmode string) (json.RawMessage, error) {
	return navPost(ctx, c, "gotoprevsection", map[string]any{"transport": ref, "playmode": playmode})
}

// GotoTrack jumps to a track.
func (c *Client) GotoTrack(ctx context.Context, ref Ref, track Ref, playmode string) (json.RawMessage, error) {
	return navPost(ctx, c, "gototrack", map[string]any{"transport": ref, "track": track, "playmode": playmode})
}

// GotoNextTrack / GotoPrevTrack step between tracks.
func (c *Client) GotoNextTrack(ctx context.Context, ref Ref, playmode string) (json.RawMessage, error) {
	return navPost(ctx, c, "gotonexttrack", map[string]any{"transport": ref, "playmode": playmode})
}
func (c *Client) GotoPrevTrack(ctx context.Context, ref Ref, playmode string) (json.RawMessage, error) {
	return navPost(ctx, c, "gotoprevtrack", map[string]any{"transport": ref, "playmode": playmode})
}

// --- State setters ---

// SetEngaged engages or disengages a transport.
func (c *Client) SetEngaged(ctx context.Context, ref Ref, engaged bool) (json.RawMessage, error) {
	return navPost(ctx, c, "engaged", map[string]any{"transport": ref, "engaged": engaged})
}

// SetVolume sets a transport's volume.
func (c *Client) SetVolume(ctx context.Context, ref Ref, volume float64) (json.RawMessage, error) {
	return navPost(ctx, c, "volume", map[string]any{"transport": ref, "volume": volume})
}

// SetBrightness sets a transport's brightness.
func (c *Client) SetBrightness(ctx context.Context, ref Ref, brightness float64) (json.RawMessage, error) {
	return navPost(ctx, c, "brightness", map[string]any{"transport": ref, "brightness": brightness})
}
