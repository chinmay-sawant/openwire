# OpenWire — Phase 1: Go Module and `openwire start` Skeleton

> **Parent:** [00-overview.md](00-overview.md)  
> **Status:** proposed; no implementation started  
> **Estimated effort:** one bootstrap slice

---

**Dependencies:** Phase 0  
**Milestone:** `openwire start --demo` builds under Go 1.26.4, opens a minimal Bubble Tea shell (dark), and exits cleanly on `q` / Ctrl+C.

## Checklist

- [ ] **P1.1 — Toolchain:** create root `go.mod` with module path (e.g. `github.com/<owner>/openwire` or local path chosen at init) and **`go 1.26.4`** / toolchain directive so builds require Go 1.26.4.
- [ ] **P1.2 — Entrypoint:** add `cmd/openwire/main.go` as the only process entry; wire Cobra (or stdlib `flag` + subcommands) there.
- [ ] **P1.3 — `start` command:** implement `openwire start` with flags:
  - `--demo` — synthetic data, no capture
  - `--iface stringSlice` — optional interface filter
  - `--theme` — default `dark`
  - `--log-level` — default `info` (logs must not break the TUI; log to file or stderr only before TUI takes over)
- [ ] **P1.4 — Makefile:** fill root `makefile` with at least: `build`, `test`, `lint`/`vet`, `run` (`go run ./cmd/openwire start --demo`).
- [ ] **P1.5 — Lifecycle:** root context cancelled on SIGINT/SIGTERM; Bubble Tea program stopped; terminal always restored (no stuck raw mode).
- [ ] **P1.6 — Minimal TUI shell:** empty dark-themed layout with title “OpenWire”, status line “demo|starting”, quit on `q`. Mouse support enabled (`tea.WithMouseCellMotion()` or current Bubble Tea equivalent).
- [ ] **P1.7 — Package layout (initial):**

```text
cmd/openwire/
internal/app/          # composition / runtime
internal/domain/       # Adapter, Flow, AppUsage, BandwidthSample
internal/store/memory/ # later phases
internal/capture/      # later phases (linux)
internal/tui/          # Bubble Tea models/views
internal/platform/     # logging, privileges helpers
```

- [ ] **P1.8 — CI-friendly smoke:** `make build && ./bin/openwire start --demo` documented; automated test can assert flag parsing and that demo mode does not open live capture devices.

**Required proof:** `go test ./...` passes; binary builds with Go 1.26.4; demo TUI starts and quits without leaving the terminal broken.
