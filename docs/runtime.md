# OpenWire Runtime Contract (v0.0.1)

## How to start

```bash
openwire start              # live capture (needs privileges on Linux)
openwire start --demo       # synthetic traffic; no privileges
openwire start --iface eth0 # pin interface(s)
openwire start --theme dark # default theme
```

Single foreground process. No daemon, no installer.

## Privileges

Live packet capture on Linux uses `AF_PACKET` raw sockets and typically requires:

- root, or
- `CAP_NET_RAW` (and often `CAP_NET_ADMIN`) on the binary

```bash
sudo setcap cap_net_raw,cap_net_admin+ep ./bin/openwire
./bin/openwire start
```

If privileges are missing, OpenWire exits with a clear error unless `--demo` is used. Live mode never silently falls back to fake data.

## Where data lives

v0.0.1 keeps flows, per-app counters, and bandwidth samples **in memory only** (bounded maps and a ring buffer). Nothing is written to SQLite or disk for traffic data.

Logs (if any) go to a file under the state directory so they do not corrupt the TUI.

## Platforms

| Platform | Capture | Notes |
|----------|---------|--------|
| Linux | Primary | Adapter discovery + AF_PACKET capture + `/proc` attribution |
| WSL2 | Supported secondary | Linux adapters + best-effort Windows host adapter listing |
| Windows / macOS native | Out of scope | v0.0.1 |

## TUI

- Bubble Tea, **dark theme by default**
- Mouse and arrow-key navigation
- Default home: apps by bandwidth + live graph

## Out of scope (v0.0.1)

Firewall rules, DNS interception, Portmaster client, Gin/React UI, system service, full packet hex dissector, SQLite persistence.
