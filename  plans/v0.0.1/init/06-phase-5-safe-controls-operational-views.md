# OpenWire — Phase 5: Safe Controls and Operational Views

> **Parent:** [00-overview.md](00-overview.md)
> **Status:** proposed; no implementation started
> **Estimated effort:** one capability-gated operations slice

---

**Dependencies:** Phase 4; explicit mutation approval from Phase 0  
**Milestone:** optional controls are safe, typed, capability-gated, and confirmed by the backend.

- [ ] **P5.1 — Capability gate:** disable or hide controls when the source is read-only or does not advertise the required capability.
- [ ] **P5.2 — Policy summary:** show global default action, filter status, DNS status, history status, and pause state before offering editors.
- [ ] **P5.3 — Minimal policy edit:** support only the approved permit/block/ask mutation with the exact profile and rule key visible before confirmation.
- [ ] **P5.4 — Rule action:** add explicit allow/block domain or IP actions only after preserving existing inbound/outbound rules and defining duplicate-rule semantics.
- [ ] **P5.5 — Mutation state:** show pending, succeeded, failed, canceled, and stale-result states; never display success before backend confirmation and refresh.
- [ ] **P5.6 — Audit:** record user-issued mutations without storing secrets; provide a safe redaction policy for logs and exported reports.
- [ ] **P5.7 — Notifications:** add notification list/unread state and interactive prompts only after the event contract supports durable IDs and resync.
- [ ] **P5.8 — History:** add paginated history search, date/filter controls, cleanup, and clear confirmation only after the schema, retention, and capability contract are stable.
- [ ] **P5.9 — Reports:** add TUI report preview and export from the structured `Report` model; do not scrape terminal output.

**Required proof:** capability, confirmation, cancellation, idempotence, duplicate-submission, refresh-after-write, pagination, notification deduplication, and backend-restart tests.
