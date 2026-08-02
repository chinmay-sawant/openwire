# OpenWire — Future Phase 7: Gin API and React Reporting

> **Parent:** [00-overview.md](00-overview.md)
> **Status:** intentionally deferred until the TUI application/read-model contract is stable
> **Estimated effort:** future transport and web-client program

---

This phase is intentionally deferred until the TUI’s application/read-model contract is stable. It is a roadmap, not part of the first TUI milestone.

- [ ] **P7.1 — Public DTOs:** move stable wire DTOs into `api/contracts/v1`, including schema version, timestamps, pagination, filtering, sorting, capabilities, and typed errors.
- [ ] **P7.2 — HTTP transport:** add `openwire serve` with Gin handlers in `internal/adapters/httpgin`; handlers call application queries/commands and never call TUI code.
- [ ] **P7.3 — Endpoints:** expose a minimal contract such as `GET /api/v1/status`, `GET /api/v1/connections`, `GET /api/v1/profiles`, `GET /api/v1/history`, `GET /api/v1/notifications`, and `GET /api/v1/reports`.
- [ ] **P7.4 — Live events:** expose `WS /api/v1/events` or SSE with bounded delivery, sequence numbers, reconnect/resync semantics, and polling fallback where appropriate.
- [ ] **P7.5 — Security:** bind loopback by default, add explicit remote-binding policy, authentication, authorization, Origin/CSRF controls, request limits, and secret-safe logs.
- [ ] **P7.6 — React client:** attach a separate React application to the versioned HTTP/event contracts for dashboards, report views, charts, and exports; do not share TUI rendering code.
- [ ] **P7.7 — Report parity:** prove that TUI, Gin, and React render the same structured report semantics from the application layer.
- [ ] **P7.8 — OpenAPI and compatibility:** publish an OpenAPI description and add contract compatibility tests before treating the API as stable.
