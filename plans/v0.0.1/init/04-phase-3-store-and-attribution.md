# OpenWire — Phase 3: In-Memory Store and Per-App Bandwidth

> **Parent:** [00-overview.md](00-overview.md)  
> **Status:** proposed; no implementation started  
> **Estimated effort:** one data-plane slice

---

**Dependencies:** Phase 2  
**Milestone:** observations become “which app uses how much network” with a short time series for graphs — all in memory.

## Goal of this phase

```text
Observation stream
       │
       ▼
  MemoryStore  ──► AppUsage ranking (bytes, rate)
       │
       └──► BandwidthSample ring (for GlassWire-style graph)
```

## Checklist

- [ ] **P3.1 — Memory store:** implement thread-safe store (`internal/store/memory`) with:
  - active flows map (key = 5-tuple + iface or normalized flow key)
  - per-app aggregate counters
  - sample ring buffer (e.g. last N seconds/minutes at fixed interval)
  - configurable caps (max flows, max samples); eviction policy documented
- [ ] **P3.2 — Ingest path:** `Ingest(Observation)` updates flow bytes, app totals, and rolling rates. Safe under concurrent capture + TUI readers.
- [ ] **P3.3 — Process attribution (Linux):** map local sockets → PID → process name/path via `/proc/net/tcp{,6}`, `/proc/net/udp{,6}`, and `/proc/<pid>/…`. Refresh mapping on an interval (not every packet).
- [ ] **P3.4 — Unknown bucket:** traffic that cannot be attributed still appears under a clear label (e.g. `unknown` / `kernel` / `other`) so totals stay honest.
- [ ] **P3.5 — Query API for TUI:**
  - `ListAppsByBandwidth(limit) []AppUsage`
  - `ListAdapters() []Adapter`
  - `Samples(since) []BandwidthSample`
  - `ListFlowsForApp(appKey, limit) []Flow`
  - `Snapshot() Status` (capture running?, privilege?, dropped packets?, iface list)
- [ ] **P3.6 — Rates:** compute bytes/sec over a short window for apps and total; graph uses samples, list uses current rate + cumulative session totals.
- [ ] **P3.7 — Demo data:** demo engine + store produce multi-app rankings and a non-flat graph without live capture.
- [ ] **P3.8 — Future storage seam:** keep a `Store` interface so SQLite can replace memory later without rewriting the TUI. Do **not** implement SQLite in v0.0.1.

**Required proof:** concurrent ingest/read race test (`go test -race`); eviction respects caps; attribution unit tests with fake `/proc` fixtures where practical; demo path shows ≥3 apps with different bandwidths.

**Acceptance criteria:** given a stream of observations, the store answers “top apps by network use” and provides a dense enough sample series to plot.
