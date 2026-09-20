# disguise-mcp

A cross-platform bridge between the **[disguise Designer API](https://developer.disguise.one/api/introduction/)**
and the **Model Context Protocol (MCP)**, so an AI client can control a disguise
media server — drive the timeline transport (play, stop, seek, sections,
tracks), read show state, and call any Designer API endpoint.

Written in Go. Single self-contained binary (the admin console is embedded) for
Windows, macOS, and Linux on amd64 and arm64. Designed to run **on the disguise
server** (Windows) and expose MCP to your network, with a small web console to
repoint it at a different disguise server on the fly.

---

## What it does

The disguise Designer API is an HTTP/REST service on the disguise server
(default **port 80**). `disguise-mcp` sits in front of it and:

- exposes MCP tools for **transport control** and Service/Session endpoints,
- serves MCP over **Streamable HTTP on all interfaces** by default, so networked
  AI clients can reach the bridge running on the disguise machine,
- runs an **admin console** on a second port to view connection status and
  **repoint** at a different disguise server (host/port) without a restart.

---

## Install

### Windows (PowerShell) — e.g. on the disguise server

```powershell
irm https://raw.githubusercontent.com/medcelerate/disguise-mcp/main/scripts/install.ps1 | iex
```

### macOS / Linux

```bash
curl -fsSL https://raw.githubusercontent.com/medcelerate/disguise-mcp/main/scripts/install.sh | sh
```

Or download a binary from the
[Releases](https://github.com/medcelerate/disguise-mcp/releases) page, or build
from source: `go install github.com/medcelerate/disguise-mcp/cmd/disguise-mcp@latest`.

---

## Quick start

```bash
cp config.example.yaml config.yaml   # edit the disguise host if needed
disguise-mcp --config config.yaml
```

Defaults: connects to disguise at `http://127.0.0.1:80`, serves **MCP over HTTP
on `0.0.0.0:8090`**, and opens the **admin console on `0.0.0.0:8091`**.

### Running on the disguise server

Install with the PowerShell one-liner, drop a `config.yaml` next to it (or rely
on defaults, since the API is local on port 80), and run `disguise-mcp`. To keep
it running, register it as a scheduled task (at logon) or a Windows service with
a wrapper such as [NSSM](https://nssm.cc/). MCP is then reachable at
`http://<disguise-server-ip>:8090` and the console at `http://<ip>:8091`.

### Connecting an MCP client

**Remote (Streamable HTTP)** — point Claude (custom connector) or any MCP client
at `http://<disguise-server-ip>:8090`.

**Local (stdio)** — set `mcp.transport: stdio` and add to a desktop MCP client:

```json
{ "mcpServers": { "disguise": { "command": "disguise-mcp", "args": ["--config", "C:\\path\\to\\config.yaml"] } } }
```

> Logs go to **stderr** so they never corrupt the stdio MCP stream on stdout.

---

## Configuration

See [`config.example.yaml`](config.example.yaml). Key fields:

| Field | Meaning |
|-------|---------|
| `disguise.host` / `disguise.port` | The disguise server's API (default `127.0.0.1:80`) |
| `disguise.scheme` | `http` or `https` |
| `mcp.transport` | `http`, `stdio`, or `both` |
| `mcp.http.addr` | MCP HTTP bind address (default `0.0.0.0:8090`) |
| `web.enabled` / `web.addr` | Admin console (default `0.0.0.0:8091`) |

Overridable via env: `DISGUISEMCP_HOST`, `DISGUISEMCP_PORT`,
`DISGUISEMCP_MCP_TRANSPORT`, `DISGUISEMCP_MCP_HTTP_ADDR`, `DISGUISEMCP_WEB_ADDR`,
`DISGUISEMCP_WEB_ENABLED`, `DISGUISEMCP_LOG_LEVEL`, `DISGUISEMCP_CONFIG`.

### Admin console

At `web.addr` (default `http://<ip>:8091`) you can see whether the disguise
server is reachable and **repoint** the bridge at a different host/port. Changes
apply immediately and are saved to the config file — the same setting the
`disguise_set_target` MCP tool changes.

---

## MCP tools

**Control:** `disguise_status`, `disguise_set_target`, `disguise_system`,
`disguise_project`, `disguise_raw` (any endpoint).

**Transport reads:** `disguise_list_transports`, `disguise_active_transport`,
`disguise_list_tracks`, `disguise_list_setlists`, `disguise_annotations`.

**Playback:** `disguise_play`, `disguise_stop`, `disguise_return_to_start`,
`disguise_play_section`, `disguise_play_loop_section`.

**Navigation:** `disguise_goto_time`, `disguise_goto_timecode`,
`disguise_goto_frame`, `disguise_goto_section`, `disguise_next_section`,
`disguise_prev_section`, `disguise_goto_track`, `disguise_next_track`,
`disguise_prev_track`.

**State:** `disguise_set_engaged`, `disguise_set_volume`,
`disguise_set_brightness`.

All tools carry titles and read-only / write annotations. Anything the dedicated
tools don't cover is reachable through `disguise_raw`.

---

## Security

The MCP endpoint and admin console bind to **all interfaces** by default because
the bridge is meant to run on a disguise server and be reached over the show
network. There is **no built-in authentication** — run it only on a trusted,
isolated production network (as is standard for disguise/show control), and set
`mcp.http.addr` / `web.addr` to `127.0.0.1:...` if you want to restrict access to
the local machine.

---

## Building from source

```bash
go build ./...   # compile
go test ./...    # run the tests
```

## License

GPL-3.0 — see [LICENSE](LICENSE).
