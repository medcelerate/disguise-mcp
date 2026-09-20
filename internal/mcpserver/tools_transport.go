package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/medcelerate/disguise-mcp/internal/disguise"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// transportIn identifies a transport for an action.
type transportIn struct {
	Transport string `json:"transport,omitempty" jsonschema:"transport name (leave blank for the active/default transport)"`
	UID       string `json:"uid,omitempty" jsonschema:"transport uid (alternative to name)"`
	Playmode  string `json:"playmode,omitempty" jsonschema:"optional playmode override"`
}

func (in transportIn) ref() disguise.Ref    { return disguise.Ref{UID: in.UID, Name: in.Transport} }
func (in transportIn) refs() []disguise.Ref { return []disguise.Ref{in.ref()} }

// registerTransportTools registers the timeline transport (playback) tools.
func registerTransportTools(s *mcp.Server, d *deps) {
	// Reads.
	mcp.AddTool(s, &mcp.Tool{Name: "disguise_list_transports",
		Description: "List all transports (timelines) and multitransports on the disguise server.",
		Annotations: annRead("List transports")}, d.listTransports)
	mcp.AddTool(s, &mcp.Tool{Name: "disguise_active_transport",
		Description: "Get the active transport(s) and their current state (playmode, current track, volume, brightness).",
		Annotations: annRead("Active transport")}, d.activeTransport)
	mcp.AddTool(s, &mcp.Tool{Name: "disguise_list_tracks",
		Description: "List the tracks available to the transport.",
		Annotations: annRead("List tracks")}, d.listTracks)
	mcp.AddTool(s, &mcp.Tool{Name: "disguise_list_setlists",
		Description: "List setlists and their tracks.",
		Annotations: annRead("List setlists")}, d.listSetlists)
	mcp.AddTool(s, &mcp.Tool{Name: "disguise_annotations",
		Description: "Get the notes, tags and sections for a transport's timeline.",
		Annotations: annRead("Transport annotations")}, d.annotations)

	// Playback.
	mcp.AddTool(s, &mcp.Tool{Name: "disguise_play",
		Description: "Play a transport (set playmode to play).",
		Annotations: annWrite("Play")}, d.play)
	mcp.AddTool(s, &mcp.Tool{Name: "disguise_stop",
		Description: "Stop a transport.",
		Annotations: annWrite("Stop")}, d.stop)
	mcp.AddTool(s, &mcp.Tool{Name: "disguise_return_to_start",
		Description: "Return a transport to the start of its timeline.",
		Annotations: annWrite("Return to start")}, d.returnToStart)
	mcp.AddTool(s, &mcp.Tool{Name: "disguise_play_section",
		Description: "Play to the end of the current section.",
		Annotations: annWrite("Play section")}, d.playSection)
	mcp.AddTool(s, &mcp.Tool{Name: "disguise_play_loop_section",
		Description: "Loop the current section.",
		Annotations: annWrite("Loop section")}, d.playLoopSection)

	// Navigation.
	mcp.AddTool(s, &mcp.Tool{Name: "disguise_goto_time",
		Description: "Seek a transport to a time in seconds.",
		Annotations: annWrite("Go to time")}, d.gotoTime)
	mcp.AddTool(s, &mcp.Tool{Name: "disguise_goto_timecode",
		Description: "Seek a transport to a timecode string (e.g. 01:00:00:00).",
		Annotations: annWrite("Go to timecode")}, d.gotoTimecode)
	mcp.AddTool(s, &mcp.Tool{Name: "disguise_goto_frame",
		Description: "Seek a transport to a frame number.",
		Annotations: annWrite("Go to frame")}, d.gotoFrame)
	mcp.AddTool(s, &mcp.Tool{Name: "disguise_goto_section",
		Description: "Jump a transport to a named section.",
		Annotations: annWrite("Go to section")}, d.gotoSection)
	mcp.AddTool(s, &mcp.Tool{Name: "disguise_next_section",
		Description: "Jump a transport to the next section.",
		Annotations: annWrite("Next section")}, d.nextSection)
	mcp.AddTool(s, &mcp.Tool{Name: "disguise_prev_section",
		Description: "Jump a transport to the previous section.",
		Annotations: annWrite("Previous section")}, d.prevSection)
	mcp.AddTool(s, &mcp.Tool{Name: "disguise_goto_track",
		Description: "Jump a transport to a track by name or uid.",
		Annotations: annWrite("Go to track")}, d.gotoTrack)
	mcp.AddTool(s, &mcp.Tool{Name: "disguise_next_track",
		Description: "Jump a transport to the next track.",
		Annotations: annWrite("Next track")}, d.nextTrack)
	mcp.AddTool(s, &mcp.Tool{Name: "disguise_prev_track",
		Description: "Jump a transport to the previous track.",
		Annotations: annWrite("Previous track")}, d.prevTrack)

	// State.
	mcp.AddTool(s, &mcp.Tool{Name: "disguise_set_engaged",
		Description: "Engage or disengage a transport.",
		Annotations: annWrite("Set engaged")}, d.setEngaged)
	mcp.AddTool(s, &mcp.Tool{Name: "disguise_set_volume",
		Description: "Set a transport's volume.",
		Annotations: annWrite("Set volume")}, d.setVolume)
	mcp.AddTool(s, &mcp.Tool{Name: "disguise_set_brightness",
		Description: "Set a transport's brightness.",
		Annotations: annWrite("Set brightness")}, d.setBrightness)
}

// --- reads ---

func (d *deps) listTransports(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, any, error) {
	raw, err := d.app.Client().Transports(ctx)
	if err != nil {
		return nil, nil, err
	}
	return jsonResult("Transports:", raw)
}

func (d *deps) activeTransport(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, any, error) {
	raw, err := d.app.Client().ActiveTransport(ctx)
	if err != nil {
		return nil, nil, err
	}
	return jsonResult("Active transport:", raw)
}

func (d *deps) listTracks(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, any, error) {
	raw, err := d.app.Client().Tracks(ctx)
	if err != nil {
		return nil, nil, err
	}
	return jsonResult("Tracks:", raw)
}

func (d *deps) listSetlists(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, any, error) {
	raw, err := d.app.Client().Setlists(ctx)
	if err != nil {
		return nil, nil, err
	}
	return jsonResult("Setlists:", raw)
}

func (d *deps) annotations(ctx context.Context, _ *mcp.CallToolRequest, in transportIn) (*mcp.CallToolResult, any, error) {
	raw, err := d.app.Client().Annotations(ctx, in.ref())
	if err != nil {
		return nil, nil, err
	}
	return jsonResult("Annotations:", raw)
}

// --- playback ---

func (d *deps) play(ctx context.Context, _ *mcp.CallToolRequest, in transportIn) (*mcp.CallToolResult, any, error) {
	return d.simple(ctx, "Play", in, d.app.Client().Play)
}
func (d *deps) stop(ctx context.Context, _ *mcp.CallToolRequest, in transportIn) (*mcp.CallToolResult, any, error) {
	return d.simple(ctx, "Stop", in, d.app.Client().Stop)
}
func (d *deps) returnToStart(ctx context.Context, _ *mcp.CallToolRequest, in transportIn) (*mcp.CallToolResult, any, error) {
	return d.simple(ctx, "Return to start", in, d.app.Client().ReturnToStart)
}
func (d *deps) playSection(ctx context.Context, _ *mcp.CallToolRequest, in transportIn) (*mcp.CallToolResult, any, error) {
	return d.simple(ctx, "Play section", in, d.app.Client().PlaySection)
}
func (d *deps) playLoopSection(ctx context.Context, _ *mcp.CallToolRequest, in transportIn) (*mcp.CallToolResult, any, error) {
	return d.simple(ctx, "Loop section", in, d.app.Client().PlayLoopSection)
}

func (d *deps) simple(ctx context.Context, label string, in transportIn, fn func(context.Context, []disguise.Ref) (json.RawMessage, error)) (*mcp.CallToolResult, any, error) {
	raw, err := fn(ctx, in.refs())
	if err != nil {
		return nil, nil, err
	}
	return jsonResult(fmt.Sprintf("%s: %s", label, targetName(in)), raw)
}

// --- navigation ---

type gotoTimeIn struct {
	transportIn
	Time float64 `json:"time" jsonschema:"time in seconds"`
}

func (d *deps) gotoTime(ctx context.Context, _ *mcp.CallToolRequest, in gotoTimeIn) (*mcp.CallToolResult, any, error) {
	raw, err := d.app.Client().GotoTime(ctx, in.ref(), in.Time, in.Playmode)
	if err != nil {
		return nil, nil, err
	}
	return jsonResult(fmt.Sprintf("Go to %.3fs on %s:", in.Time, targetName(in.transportIn)), raw)
}

type gotoTimecodeIn struct {
	transportIn
	Timecode   string `json:"timecode" jsonschema:"timecode, e.g. 01:00:00:00"`
	IgnoreTags bool   `json:"ignoreTags,omitempty" jsonschema:"ignore timecode tags when seeking"`
}

func (d *deps) gotoTimecode(ctx context.Context, _ *mcp.CallToolRequest, in gotoTimecodeIn) (*mcp.CallToolResult, any, error) {
	raw, err := d.app.Client().GotoTimecode(ctx, in.ref(), in.Timecode, in.IgnoreTags, in.Playmode)
	if err != nil {
		return nil, nil, err
	}
	return jsonResult(fmt.Sprintf("Go to timecode %s on %s:", in.Timecode, targetName(in.transportIn)), raw)
}

type gotoFrameIn struct {
	transportIn
	Frame int `json:"frame" jsonschema:"frame number"`
}

func (d *deps) gotoFrame(ctx context.Context, _ *mcp.CallToolRequest, in gotoFrameIn) (*mcp.CallToolResult, any, error) {
	raw, err := d.app.Client().GotoFrame(ctx, in.ref(), in.Frame, in.Playmode)
	if err != nil {
		return nil, nil, err
	}
	return jsonResult(fmt.Sprintf("Go to frame %d on %s:", in.Frame, targetName(in.transportIn)), raw)
}

type gotoSectionIn struct {
	transportIn
	Section string `json:"section" jsonschema:"section name"`
}

func (d *deps) gotoSection(ctx context.Context, _ *mcp.CallToolRequest, in gotoSectionIn) (*mcp.CallToolResult, any, error) {
	raw, err := d.app.Client().GotoSection(ctx, in.ref(), in.Section, in.Playmode)
	if err != nil {
		return nil, nil, err
	}
	return jsonResult(fmt.Sprintf("Go to section %q on %s:", in.Section, targetName(in.transportIn)), raw)
}

func (d *deps) nextSection(ctx context.Context, _ *mcp.CallToolRequest, in transportIn) (*mcp.CallToolResult, any, error) {
	raw, err := d.app.Client().GotoNextSection(ctx, in.ref(), in.Playmode)
	if err != nil {
		return nil, nil, err
	}
	return jsonResult("Next section:", raw)
}

func (d *deps) prevSection(ctx context.Context, _ *mcp.CallToolRequest, in transportIn) (*mcp.CallToolResult, any, error) {
	raw, err := d.app.Client().GotoPrevSection(ctx, in.ref(), in.Playmode)
	if err != nil {
		return nil, nil, err
	}
	return jsonResult("Previous section:", raw)
}

type gotoTrackIn struct {
	transportIn
	Track    string `json:"track" jsonschema:"track name"`
	TrackUID string `json:"trackUid,omitempty" jsonschema:"track uid (alternative to name)"`
}

func (d *deps) gotoTrack(ctx context.Context, _ *mcp.CallToolRequest, in gotoTrackIn) (*mcp.CallToolResult, any, error) {
	raw, err := d.app.Client().GotoTrack(ctx, in.ref(), disguise.Ref{UID: in.TrackUID, Name: in.Track}, in.Playmode)
	if err != nil {
		return nil, nil, err
	}
	return jsonResult(fmt.Sprintf("Go to track %q:", in.Track), raw)
}

func (d *deps) nextTrack(ctx context.Context, _ *mcp.CallToolRequest, in transportIn) (*mcp.CallToolResult, any, error) {
	raw, err := d.app.Client().GotoNextTrack(ctx, in.ref(), in.Playmode)
	if err != nil {
		return nil, nil, err
	}
	return jsonResult("Next track:", raw)
}

func (d *deps) prevTrack(ctx context.Context, _ *mcp.CallToolRequest, in transportIn) (*mcp.CallToolResult, any, error) {
	raw, err := d.app.Client().GotoPrevTrack(ctx, in.ref(), in.Playmode)
	if err != nil {
		return nil, nil, err
	}
	return jsonResult("Previous track:", raw)
}

// --- state ---

type engagedIn struct {
	transportIn
	Engaged bool `json:"engaged" jsonschema:"true to engage, false to disengage"`
}

func (d *deps) setEngaged(ctx context.Context, _ *mcp.CallToolRequest, in engagedIn) (*mcp.CallToolResult, any, error) {
	raw, err := d.app.Client().SetEngaged(ctx, in.ref(), in.Engaged)
	if err != nil {
		return nil, nil, err
	}
	return jsonResult(fmt.Sprintf("Set engaged=%v:", in.Engaged), raw)
}

type volumeIn struct {
	transportIn
	Volume float64 `json:"volume" jsonschema:"volume level"`
}

func (d *deps) setVolume(ctx context.Context, _ *mcp.CallToolRequest, in volumeIn) (*mcp.CallToolResult, any, error) {
	raw, err := d.app.Client().SetVolume(ctx, in.ref(), in.Volume)
	if err != nil {
		return nil, nil, err
	}
	return jsonResult(fmt.Sprintf("Set volume=%v:", in.Volume), raw)
}

type brightnessIn struct {
	transportIn
	Brightness float64 `json:"brightness" jsonschema:"brightness level"`
}

func (d *deps) setBrightness(ctx context.Context, _ *mcp.CallToolRequest, in brightnessIn) (*mcp.CallToolResult, any, error) {
	raw, err := d.app.Client().SetBrightness(ctx, in.ref(), in.Brightness)
	if err != nil {
		return nil, nil, err
	}
	return jsonResult(fmt.Sprintf("Set brightness=%v:", in.Brightness), raw)
}

// helpers

func targetName(in transportIn) string {
	if in.Transport != "" {
		return in.Transport
	}
	if in.UID != "" {
		return "uid " + in.UID
	}
	return "active transport"
}
