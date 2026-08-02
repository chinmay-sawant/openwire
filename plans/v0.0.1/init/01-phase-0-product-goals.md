# OpenWire — Phase 0: Product Goals and Runtime Contract

> **Parent:** [00-overview.md](00-overview.md)  
> **Status:** proposed; no implementation started  
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

- [ ] **P0.1 — Primary command:** lock `openwire start` as the only required user command for the first usable product. Optional flags: `--demo`, `--iface`, `--theme` (default dark), `--log-level`.
- [ ] **P0.2 — Privilege contract:** document that live capture on Linux typically needs root or `CAP_NET_RAW`/`CAP_NET_ADMIN`. Capture failures must be explicit errors, not silent empty graphs. Demo mode is opt-in only.
- [ ] **P0.3 — Platform matrix:** v0.0.1 **supports Linux** as the primary capture target. WSL2 is a first-class secondary environment (Phase 5). Native Windows/macOS capture is out of scope for this version.
- [ ] **P0.4 — Data model sketch:** define stable names for:
  - `Adapter` (name, index, MAC, IPs, up/down, rx/tx totals)
  - `Flow` / `Connection` (5-tuple, protocol, bytes in/out, first/last seen, pid if known)
  - `AppUsage` (process name, pid, path if known, bytes in/out, rate)
  - `BandwidthSample` (timestamp, total rx/tx, optional per-app rates for the graph)
- [ ] **P0.5 — Storage contract:** v0.0.1 is **in-memory only** with explicit bounds (max flows, max samples, max apps). Document the eviction policy (e.g. LRU flows, ring buffer for samples). Future SQLite is a non-goal for this milestone.
- [ ] **P0.6 — TUI contract:** Bubble Tea; dark theme default; panes for graph, app list, detail/status; mouse enable on startup; arrows/tab/enter/esc/q keyboard map.
- [ ] **P0.7 — Non-goals freeze:** no firewall rules, no DNS hijack, no Portmaster API client, no Gin/React UI, no system service, no packet hex dump UI in v0.0.1.
- [ ] **P0.8 — README obligation:** Phase 0 decisions must land in the root `README.md` before Phase 6 closes (draft can start earlier).

**Acceptance criteria:** a short `docs/runtime.md` (or README section) answers: how to start, what privileges, where data lives, how demo differs from live, and what is out of scope.
