# OpenWire — Phase 2: Domain and Application Architecture

> **Parent:** [00-overview.md](00-overview.md)
> **Status:** proposed; no implementation started
> **Estimated effort:** one contract and boundary slice

---

**Dependencies:** Phase 1  
**Milestone:** the TUI can consume a source through typed application ports without knowing its transport or storage implementation.

### Proposed package layout

```text
cmd/openwire/
internal/domain/
  connection.go
  profile.go
  status.go
  notification.go
  report.go
  errors.go
internal/application/
  ports.go
  queries.go
  commands.go
  readmodel/
  events.go
internal/adapters/source/demo/
internal/adapters/source/portmaster/
internal/adapters/tui/
  model.go
  messages.go
  views/
internal/platform/config/
internal/platform/logging/
api/contracts/v1/                 # introduce when the public contract is frozen
```

- [ ] **P2.1 — Domain models:** define immutable, JSON-friendly domain read models with timestamps, stable IDs, capability metadata, and redaction rules.
- [ ] **P2.2 — Application ports:** define typed interfaces for status snapshots, connection queries, profile reads, notification reads, report assembly, and live event subscription.
- [ ] **P2.3 — Application facade:** implement use cases that accept `context.Context`, return typed results, and classify errors without exposing transport details.
- [ ] **P2.4 — Event semantics:** define revision/sequence numbers, snapshot-versus-delta behavior, bounded queues, dropped-event detection, and resync after reconnect or overflow.
- [ ] **P2.5 — Capability discovery:** represent unsupported features explicitly instead of making the TUI infer capability from missing fields or endpoint failures.
- [ ] **P2.6 — Report model:** define a versioned structured report containing generated time, status, connection summaries, notifications, metrics, and warnings; keep terminal formatting out of the model.
- [ ] **P2.7 — Fake source:** add a controllable source that can emit snapshots, updates, deletions, duplicates, stale events, disconnects, reconnects, malformed data, and cancellation.
- [ ] **P2.8 — Ownership tests:** assert that application/domain packages do not import TUI, Gin, browser, raw Portmaster records, or platform-specific firewall packages.

**Required proof:** contract tests shared by demo and fake sources; event ordering, deduplication, deletion, resync, cancellation, backpressure, and leak/race tests.
