# OpenWire — Phase 1: Go Module and Command Skeleton

> **Parent:** [00-overview.md](00-overview.md)
> **Status:** proposed; no implementation started
> **Estimated effort:** one executable/bootstrap slice

---

**Dependencies:** Phase 0  
**Milestone:** `openwire tui --demo` starts and exits cleanly.

- [ ] **P1.1 — Module:** create the root `go.mod` with the OpenWire module path and pinned, minimal dependencies.
- [ ] **P1.2 — Entrypoint:** create `cmd/openwire/main.go` as the composition root; keep flag parsing and dependency wiring out of `internal/tui`.
- [ ] **P1.3 — Command:** add the `tui` subcommand and `--demo`, `--endpoint`, `--api-key`/credential-file, `--refresh`, `--config`, `--no-color`, and `--log-level` options as approved in Phase 0.
- [ ] **P1.4 — Configuration:** implement typed configuration loading and validation with the documented precedence rules; reject remote cleartext endpoints unless explicitly allowed by a future policy.
- [ ] **P1.5 — Lifecycle:** add context cancellation, SIGINT/SIGTERM handling, bounded shutdown, and terminal restoration on normal and error paths.
- [ ] **P1.6 — Errors:** define stable application error categories and process exit codes for configuration, authentication, unavailable backend, malformed payload, and internal failures.
- [ ] **P1.7 — Demo source:** add deterministic seeded data for status, connections, profiles, and notifications so screenshots and tests are repeatable.
- [ ] **P1.8 — Build workflow:** add root `makefile` targets for `build`, `test`, `lint`, `vet`, and a demo smoke check; use exact commands in the plan closure record.

**Required proof:** configuration unit tests, exit-code tests, demo smoke test, and a terminal restoration test. For this and every later non-documentation phase, run both `make lint` and `make test` before closing rows.
