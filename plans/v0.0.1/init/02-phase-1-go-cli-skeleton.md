# OpenWire — Phase 1: Go Module and `openwire start` Skeleton

> **Parent:** [00-overview.md](00-overview.md)  
> **Status:** implemented  
> **Estimated effort:** one bootstrap slice

---

**Dependencies:** Phase 0  
**Milestone:** `openwire start --demo` builds under Go 1.26.4, opens a minimal Bubble Tea shell (dark), and exits cleanly on `q` / Ctrl+C.

## Checklist

- [x] **P1.1 — Toolchain:** `go.mod` with `go 1.26.4`, module `github.com/chinmay-sawant/openwire`.
- [x] **P1.2 — Entrypoint:** `cmd/openwire/main.go` with Cobra.
- [x] **P1.3 — `start` command:** `--demo`, `--iface`, `--theme`, `--log-level`.
- [x] **P1.4 — Makefile:** `build`, `test`, `vet`/`lint`, `run`, `clean`.
- [x] **P1.5 — Lifecycle:** signal.NotifyContext + tea.WithContext; alt screen restored on quit.
- [x] **P1.6 — Minimal TUI shell:** evolved into full dark layout (Phase 4).
- [x] **P1.7 — Package layout:** cmd/openwire, internal/{app,domain,store/memory,capture,tui,platform}.
- [x] **P1.8 — Smoke:** `make build`; live without priv fails clearly; demo starts TUI.

**Required proof:** `go test ./...` passes; binary builds with Go 1.26.4.
