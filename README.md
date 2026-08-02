# OpenWire

Terminal network monitor: capture local traffic (Wireshark-style observation), keep it in memory, and show **which applications use how much bandwidth** in a **GlassWire-like Bubble Tea UI**.

> Status: **planned** — see [`plans/v0.0.1/init/`](plans/v0.0.1/init/00-overview.md). Implementation has not started.

## Goals (v0.0.1)

| Goal | Detail |
|------|--------|
| One command | `openwire start` opens the UI |
| Capture | Monitor traffic on local network adapters (Linux first) |
| Store | In-memory only for now (SQLite later) |
| Default view | Apps ranked by network use + live bandwidth graph |
| TUI | Bubble Tea, **dark theme by default** |
| Input | **Mouse** and **arrow keys** |
| WSL2 | Detect Windows host adapters; honest limits on Windows process attribution |

## Requirements

- **Go 1.26.4**
- **Linux** for live capture (primary target)
- **libpcap** (or documented AF_PACKET path) and capture privileges for live mode  
  Typically root, or capabilities such as `CAP_NET_RAW` / `CAP_NET_ADMIN`
- A terminal that supports colors and (optionally) mouse events

## Quick start (once implemented)

```bash
# Build
make build

# Demo UI — no root, synthetic traffic
./bin/openwire start --demo

# Live monitor (needs privileges + adapters)
sudo ./bin/openwire start

# Optional: pin interfaces
sudo ./bin/openwire start --iface eth0 --iface wlan0
```

## How it works

```text
adapters ──► capture ──► in-memory store ──► per-app bandwidth
                              │
                              └──► Bubble Tea UI (graph + app list)
```

1. **Discover** network interfaces on Linux.
2. **Capture** packets/bytes on selected interfaces.
3. **Store** flows and samples in a bounded in-memory store.
4. **Attribute** sockets to processes via `/proc` where possible.
5. **Render** a dark TUI: live graph, app list, detail pane; navigate with mouse or keys.

## UI (target)

- Dark theme by default  
- Bandwidth graph over a recent time window (GlassWire-style)  
- Application list sorted by usage  
- Status bar with adapters and capture state  
- Keys: arrows / Tab / Enter / Esc / `q`  
- Mouse: click to select, scroll lists, focus panes  

## WSL2

When OpenWire detects WSL2 it will:

- List **Linux** interfaces as usual  
- Attempt to list **Windows host adapters** (e.g. via `powershell.exe` / host commands) and show them labeled as host adapters  

**Limits:** Windows applications are not Linux PIDs. Per-app attribution for Windows processes may be incomplete or unavailable inside WSL2; the UI and docs will not pretend otherwise. Host-side counters may be used when available.

## Non-goals (v0.0.1)

- Firewall / allow–block policies  
- DNS interception  
- Full packet-dissector UI (Wireshark expert mode)  
- SQLite / disk persistence  
- Web UI (Gin + React)  
- Portmaster API client  
- System service installer / tray app  
- Native Windows or macOS capture engines (beyond WSL2 host adapter visibility)

## Project layout (planned)

```text
cmd/openwire/           # CLI entry — openwire start
internal/domain/        # Adapter, Flow, AppUsage, samples
internal/capture/       # Linux capture + demo engine
internal/store/memory/  # In-memory store
internal/tui/           # Bubble Tea views
plans/v0.0.1/init/      # Phase-wise implementation plan
```

## Plans

Canonical plan: **[plans/v0.0.1/init/00-overview.md](plans/v0.0.1/init/00-overview.md)**

| Phase | Focus |
|-------|--------|
| 0 | Product goals and runtime contract |
| 1 | Go 1.26.4 module + `openwire start` skeleton |
| 2 | Adapter discovery + capture (Linux) |
| 3 | In-memory store + per-app attribution |
| 4 | Bubble Tea UI (dark, mouse, graphs) |
| 5 | WSL2, README polish, release gates |

## License

TBD.
