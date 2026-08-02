# OpenWire

Terminal network monitor: capture local traffic (Wireshark-style observation), keep it in memory, and show **which applications use how much bandwidth** in a **GlassWire-like Bubble Tea UI**.

## Requirements

- **Go 1.26.4** (toolchain auto-download via `GOTOOLCHAIN=go1.26.4` if needed)
- **Linux** for live capture (primary target; WSL2 supported as secondary)
- Capture privileges for live mode: root or `CAP_NET_RAW` / `CAP_NET_ADMIN`
- Terminal with color support (mouse optional but supported)

No libpcap/CGO: live capture uses pure-Go **AF_PACKET**.

## Quick start

```bash
# Build
make build

# Demo UI — synthetic traffic
./bin/openwire start --demo

# Default: AF_PACKET if privileged, else unprivileged /proc stats mode
./bin/openwire start

# Full packet capture (needs privileges)
sudo ./bin/openwire start
# or require privileges and fail otherwise:
./bin/openwire start --strict-capture

# Optional: pin interfaces
sudo ./bin/openwire start --iface eth0

# SQLite history (default: ~/.local/state/openwire/openwire.db)
./bin/openwire start --demo --db /tmp/openwire.db
./bin/openwire start --no-db

# Packet archive (Wireshark) — needs live AF_PACKET privileges
sudo ./bin/openwire start --pcap /tmp/openwire.pcap --pcap-max-mb 128

# Deep attribution (conntrack) + optional eBPF when AF_PACKET is unavailable
sudo ./bin/openwire start --deep --ebpf
```

Or with capabilities instead of full root:

```bash
make build
sudo setcap cap_net_raw,cap_net_admin+ep ./bin/openwire
./bin/openwire start
```

Privileged unit proof (optional):

```bash
make test-live   # docker + CAP_NET_RAW; skips/no-ops if docker unavailable
```

## Features (v0.0.1)

| Feature | Status |
|---------|--------|
| `openwire start` one-command UI | yes |
| Dark theme default | yes |
| Mouse + arrow-key navigation | yes |
| Linux adapter discovery | yes |
| Live AF_PACKET capture | yes (needs privileges) |
| Unprivileged `/proc` stats mode | yes (default fallback) |
| Deep attribution (nf_conntrack) | yes (`--deep` / auto when no AF_PACKET) |
| eBPF kprobe counters | best-effort (`--ebpf`, needs privileges) |
| PCAP packet archive | yes (`--pcap`, live AF_PACKET) |
| In-memory store (bounded) | yes |
| Per-app bandwidth ranking | yes (`/proc` attribution on Linux) |
| GlassWire-style sparkline graph | yes |
| Demo mode | yes (`--demo`) |
| WSL2 host adapter listing | best-effort via `powershell.exe` / `ipconfig.exe` |
| WSL2 host byte counters | best-effort |
| WSL2 Windows per-process usage | best-effort connection-weighted (`win/<name>`) |
| SQLite history (samples + app snapshots) | yes (`--db` / default state dir; `--no-db` to disable) |
| Firewall / DNS control | no (non-goal) |

## How it works

```text
adapters ──► capture (demo | AF_PACKET) ──► in-memory store ──► per-app bandwidth
                                                    │
                                                    └──► Bubble Tea UI
```

1. Discover network interfaces on Linux (and Windows host adapters under WSL2 when possible).
2. Capture packets on selected interfaces **or** emit synthetic demo traffic.
3. Store flows and samples in a bounded in-memory store.
4. Attribute sockets to processes via `/proc` (Linux live mode).
5. Render dark TUI: live graph, app list, detail pane.

## UI controls

| Input | Action |
|-------|--------|
| `↑` / `↓` or `j` / `k` | Move selection |
| Mouse click / scroll | Select apps / focus panes |
| `Tab` | Cycle panes |
| `Enter` | Detail focus |
| `Esc` | Back to app list |
| `q` / `Ctrl+C` | Quit |

## WSL2

When OpenWire detects WSL2 it will:

- Capture/sample on **Linux** interfaces (AF_PACKET or `/proc` stats)
- List **Windows host adapters** (labeled `win:…`)
- Sample host adapter **byte counters**
- Attribute host traffic to Windows processes as **`win/<name>`** by open-connection weight (fallback aggregate app `windows-host`)

**Limits:** Attribution is connection-weighted host NIC deltas (not ETW packet capture). Linux process attribution still applies only to Linux processes.

See [`docs/runtime.md`](docs/runtime.md) for the full runtime contract.

## Development

```bash
make test
make vet
make run    # go run … start --demo
```

Module path: `github.com/chinmay-sawant/openwire`  
Plan: [`plans/v0.0.1/init/00-overview.md`](plans/v0.0.1/init/00-overview.md)

## Project layout

```text
cmd/openwire/              # CLI entry — openwire start
internal/domain/           # Adapter, Flow, AppUsage, samples
internal/capture/          # Demo + Linux AF_PACKET + /proc attribution
internal/store/memory/     # In-memory store
internal/tui/              # Bubble Tea views
internal/app/              # Composition / runtime
internal/platform/         # logging, WSL helpers
docs/runtime.md            # Runtime contract
plans/v0.0.1/init/         # Phase-wise plan
```

## Non-goals (v0.0.1)

- Firewall / allow–block policies  
- DNS interception  
- Full packet-dissector UI  
- SQLite / disk persistence  
- Web UI  
- Portmaster API client  
- System service installer  

## License

TBD.
