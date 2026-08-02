# OpenWire Runtime Contract (v0.0.1)

## How to start

```bash
openwire start                   # packet capture if privileged, else /proc stats mode
openwire start --strict-capture  # require AF_PACKET; error if no CAP_NET_RAW
openwire start --demo            # synthetic traffic; no privileges
openwire start --iface eth0      # pin interface(s)
openwire start --theme dark      # default theme
```

Single foreground process. No daemon, no installer.

## Capture modes

| Mode | How selected | What it does |
|------|----------------|--------------|
| `live` | AF_PACKET open succeeds | Full packet observation + `/proc` process attribution |
| `stats` | AF_PACKET fails with privilege error (default fallback) | `/proc/net/dev` byte deltas + socket→process weights (best-effort) |
| `demo` | `--demo` | Synthetic multi-app traffic |

`--strict-capture` disables the `stats` fallback. Demo data is **never** used unless `--demo` is set.

## Privileges

Full packet capture on Linux uses `AF_PACKET` raw sockets and typically requires:

- root, or
- `CAP_NET_RAW` (and often `CAP_NET_ADMIN`) on the binary

```bash
sudo setcap cap_net_raw,cap_net_admin+ep ./bin/openwire
./bin/openwire start
```

Without privileges, OpenWire enters **stats mode** (clearly labeled in the UI) unless `--strict-capture` is set.

## Where data lives

- **Live UI data** stays in a bounded **in-memory** store (flows, app counters, sample ring).
- **SQLite history** (v0.0.2+) optionally persists bandwidth samples and periodic app snapshots:
  - default path: `$XDG_STATE_HOME/openwire/openwire.db` (or `~/.local/state/openwire/openwire.db`)
  - override with `--db PATH`
  - disable with `--no-db`
  - on start, recent samples are loaded into the graph ring

Logs go to a file under the same state directory so they do not corrupt the TUI.

## Platforms

| Platform | Capture | Notes |
|----------|---------|--------|
| Linux | Primary | Adapter discovery + AF_PACKET or `/proc` stats + process attribution |
| WSL2 | Supported secondary | Linux path + host adapters + connection-weighted Windows process apps (`win/<name>`) |
| Windows / macOS native | Out of scope | no native agent yet |

### WSL2 host traffic

- Host adapters are listed via PowerShell / `ipconfig` when available.
- Host **byte counters** are sampled via `Get-NetAdapterStatistics`.
- **Per-process Windows attribution (best-effort):** list Windows TCP/UDP endpoints (`Get-NetTCPConnection` / `Get-NetUDPEndpoint`) and process names, then distribute each interval’s host NIC byte **delta** across those processes by **open-connection weight**. Apps appear as `win/<ProcessName>` (Windows PID in detail).
- If the process list is unavailable, traffic is attributed to the aggregate app `windows-host`.
- This is **not** packet-accurate ETW/WFP accounting; it is a practical WSL2 estimate without a native Windows agent.

## TUI

- Bubble Tea, **dark theme by default**
- Mouse and arrow-key navigation
- Default home: apps by bandwidth + live graph

## Out of scope (still)

Firewall rules, DNS interception, Portmaster client, Gin/React UI, system service, full packet hex dissector, native Windows ETW agent, eBPF deep attribution.
