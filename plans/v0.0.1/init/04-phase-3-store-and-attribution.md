# OpenWire — Phase 3: In-Memory Store and Per-App Bandwidth

> **Parent:** [00-overview.md](00-overview.md)  
> **Status:** implemented  
> **Estimated effort:** one data-plane slice

---

**Dependencies:** Phase 2  
**Milestone:** observations become “which app uses how much network” with a short time series for graphs — all in memory.

## Checklist

- [x] **P3.1 — Memory store:** `internal/store/memory` with flow map, app aggregates, sample ring, caps.
- [x] **P3.2 — Ingest path:** concurrent-safe `Ingest`.
- [x] **P3.3 — Process attribution (Linux):** `/proc/net/*` + inode→pid via `/proc/*/fd` (`Attributor`).
- [x] **P3.4 — Unknown bucket:** traffic without attribution labeled `unknown`.
- [x] **P3.5 — Query API:** ListAppsByBandwidth, ListAdapters, Samples, ListFlowsForApp, Snapshot.
- [x] **P3.6 — Rates:** per-app and total rates from sample deltas.
- [x] **P3.7 — Demo data:** multi-app ranking + graph samples.
- [x] **P3.8 — Future storage seam:** Store methods on concrete type; SQLite not implemented (intentional).

**Required proof:** store unit tests + race test on memory package.
