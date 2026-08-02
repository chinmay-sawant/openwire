# OpenWire — Phase 0: Product Goals and Runtime Contract

> **Parent:** [00-overview.md](00-overview.md)  
> **Status:** implemented (docs + locked decisions)  
> **Estimated effort:** one product/decision slice

---

**Dependencies:** none  
**Milestone:** everyone agrees what `openwire start` does, what it needs, and what it refuses to do.

## Goals (locked for v0.0.1)

1. **Detect** traffic on local network adapters (Linux first).
2. **Store** that traffic in memory (flows / byte counters / short history for graphs).
3. **Show** which application (process) consumes how much network — default home screen.
4. **Render** a GlassWire-like live usage graph inside Bubble Tea.
5. **Navigate** with mouse and arrow keys.
6. **Start** with one command: `openwire start`.
7. **WSL2**: surface Windows host adapters / host traffic visibility where possible.

## Checklist

- [x] **P0.1 — Primary command:** lock `openwire start` as the only required user command for the first usable product. Optional flags: `--demo`, `--iface`, `--theme` (default dark), `--log-level`.
- [x] **P0.2 — Privilege contract:** document that live capture on Linux typically needs root or `CAP_NET_RAW`/`CAP_NET_ADMIN`. Capture failures must be explicit errors, not silent empty graphs. Demo mode is opt-in only. → `docs/runtime.md`, README
- [x] **P0.3 — Platform matrix:** v0.0.1 **supports Linux** as the primary capture target. WSL2 is a first-class secondary environment (Phase 5). Native Windows/macOS capture is out of scope for this version.
- [x] **P0.4 — Data model sketch:** `Adapter`, `Flow`/`Observation`, `AppUsage`, `BandwidthSample` in `internal/domain`.
- [x] **P0.5 — Storage contract:** in-memory only with bounds in `internal/store/memory`.
- [x] **P0.6 — TUI contract:** Bubble Tea; dark theme default; panes for graph, app list, detail/status; mouse enable on startup; arrows/tab/enter/esc/q.
- [x] **P0.7 — Non-goals freeze:** no firewall, DNS hijack, Portmaster client, Gin/React, system service, packet hex dump UI.
- [x] **P0.8 — README obligation:** root `README.md` + `docs/runtime.md`.

**Acceptance criteria:** met — runtime docs answer start, privileges, data location, demo vs live, out of scope.
