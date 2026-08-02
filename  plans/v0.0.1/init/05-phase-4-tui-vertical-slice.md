# OpenWire — Phase 4: TUI Vertical Slice

> **Parent:** [00-overview.md](00-overview.md)
> **Status:** proposed; no implementation started
> **Estimated effort:** one usable-monitor slice

---

**Dependencies:** Phase 2; Phase 3 for live mode  
**Milestone:** a user can launch demo mode, inspect and filter activity, open a detail view, and quit safely.

- [ ] **P4.1 — Root model:** implement loading, ready, offline, unauthorized, stale, reconnecting, unsupported, and fatal-error states.
- [ ] **P4.2 — Navigation:** implement keyboard navigation, page focus, selection, refresh, help, and quit actions without embedding application logic in key handlers.
- [ ] **P4.3 — Status bar:** show source mode, service status, last successful sync, stale/reconnecting state, and read-only/capability indicators.
- [ ] **P4.4 — Dashboard:** show summary counts, current verdict distribution, active connection count, and the most recent notifications from structured read models.
- [ ] **P4.5 — Activity table:** show application/profile, destination, port, protocol, verdict, direction, time, and byte counters with bounded rows.
- [ ] **P4.6 — Filtering:** support the MVP fields `app/profile`, `domain`, `ip`, `country`, `verdict`, `active`, `tunneled`, and free-text search; keep parsing separate from source-specific syntax.
- [ ] **P4.7 — Sorting:** support time, application, destination, verdict, and bytes with stable ordering and clear sort indicators.
- [ ] **P4.8 — Details:** show full timestamps, executable path, local/remote endpoints, protocol, encryption, verdict, reason, DNS metadata, and optional SPN metadata when supplied.
- [ ] **P4.9 — Profiles:** add a read-only profile browser with search, active-connection count, and effective-versus-default settings summary.
- [ ] **P4.10 — Terminal resilience:** handle minimum/normal/wide terminal sizes, no-color terminals, slow sources, backend restarts, and bounded event retention.
- [ ] **P4.11 — Rendering tests:** add deterministic golden/snapshot coverage for dashboard, table, detail, loading, offline, unauthorized, stale, error, and terminal-too-small states.
- [ ] **P4.12 — Demo acceptance:** add an end-to-end demo check proving launch, navigation, filtering, detail inspection, and clean exit without an external service.

**Acceptance criteria:** `openwire tui --demo` is usable without network access; live mode uses the same TUI model; a backend outage produces a recoverable offline state rather than a panic; the process does not leave the terminal in raw mode.
