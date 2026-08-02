# OpenWire — Phase 3: Portmaster-Compatible Read-Only Source Adapter

> **Parent:** [00-overview.md](00-overview.md)
> **Status:** proposed; no implementation started
> **Estimated effort:** one compatibility transport slice

---

**Dependencies:** Phase 2  
**Milestone:** the TUI can show live data from a compatible local Portmaster endpoint without importing Portmaster’s Go module.

This phase is an adapter and compatibility exercise, not an attempt to reproduce Portmaster’s privileged service inside OpenWire.

- [ ] **P3.1 — HTTP transport:** implement endpoint normalization, `/api/v1/` path construction, request timeouts, context cancellation, bounded response bodies, and non-2xx handling.
- [ ] **P3.2 — Authentication:** implement the approved API-key/session mechanism without putting keys or cookies in logs, screen messages, URLs, or panic output.
- [ ] **P3.3 — Readiness:** implement a typed health/status probe and map connection, unauthorized, unavailable, and malformed responses to application errors.
- [ ] **P3.4 — Status mapping:** translate the reference system status into OpenWire’s `SystemStatus` read model.
- [ ] **P3.5 — Connection mapping:** translate network query records into validated `Connection` values; keep raw database keys and record types inside the adapter.
- [ ] **P3.6 — Profile mapping:** add read-only profile listing/search only after the profile identifier and field mapping are covered by fixtures.
- [ ] **P3.7 — Live events:** implement WebSocket subscription or a bounded polling fallback, with reconnect backoff, resubscription, revision tracking, and explicit resync.
- [ ] **P3.8 — Transport security:** default to loopback; reject or prominently gate non-loopback cleartext endpoints; require explicit policy for remote use and Origin behavior.
- [ ] **P3.9 — Fixture corpus:** add canonical HTTP/WebSocket payload fixtures for success, empty results, authentication failure, malformed fields, oversized responses, reconnect, and partial updates.

**Required proof:** `httptest.Server` tests, local WebSocket fake-server tests, reconnect/resubscription tests, bounded-payload tests, race tests, and credential-redaction assertions.
