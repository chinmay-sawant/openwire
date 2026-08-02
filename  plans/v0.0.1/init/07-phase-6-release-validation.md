# OpenWire — Phase 6: Release and Validation Closure

> **Parent:** [00-overview.md](00-overview.md)
> **Status:** proposed; no implementation started
> **Estimated effort:** one release-readiness slice

---

**Dependencies:** Phases 1–5  
**Milestone:** reproducible local validation for the TUI with clear limits on platform and privilege claims.

- [ ] **P6.1 — Unit gate:** run `make test` and record the current result in this checklist.
- [ ] **P6.2 — Lint gate:** run `make lint` and record the current result in this checklist.
- [ ] **P6.3 — Static gate:** run `go vet ./...` and record the current result.
- [ ] **P6.4 — Race gate:** run `go test -race ./...` for source subscriptions, reducers, and shutdown paths.
- [ ] **P6.5 — Build gate:** build the approved target OS/architectures and run the demo smoke check from a fresh checkout.
- [ ] **P6.6 — Secret-safety gate:** prove that credentials are absent from normal logs, error strings, terminal rendering, fixture snapshots, and reports.
- [ ] **P6.7 — Resource gate:** prove bounded connection/event/history/notification memory and cancellation of all transport goroutines.
- [ ] **P6.8 — Platform gate:** document supported terminals and OS targets; fail clearly on unsupported platforms rather than implying firewall or service support.
- [ ] **P6.9 — Scope gate:** verify that the TUI does not import or invoke firewall interception, driver, service-install, updater, or privileged-system code.
- [ ] **P6.10 — Checklist closure:** record exact commands, environment, dataset/fixtures, and outcomes; leave any unproven row unchecked or mark it `[~]` with an owner and next gate.
