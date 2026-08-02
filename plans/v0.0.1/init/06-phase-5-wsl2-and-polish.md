# OpenWire — Phase 5: WSL2 Awareness, README, and Release Gates

> **Parent:** [00-overview.md](00-overview.md)  
> **Status:** implemented (including pending P5.4/P5.14 follow-ups)  
> **Estimated effort:** one environment + documentation + validation slice

---

**Dependencies:** Phases 1–4  
**Milestone:** Linux path solid; WSL2 honest; README matches product; quality gates pass.

## Checklist

### WSL2 / Windows adapter visibility

- [x] **P5.1 — Environment detect:** `wsl.IsWSL2()`.
- [x] **P5.2 — Host adapter listing:** PowerShell / ipconfig best-effort.
- [x] **P5.3 — UI labeling:** `win:` prefix for windows-host adapters; WSL2 badge in header.
- [x] **P5.4 — Traffic path:**
  - Linux: AF_PACKET when privileged; else `/proc` stats mode (labeled, not silent demo).
  - WSL2 host: adapter inventory + `Get-NetAdapterStatistics` counters ingested as synthetic app `windows-host`.
  - Windows **per-process** attribution remains future work (native helper). Documented in README + `docs/runtime.md`.
- [x] **P5.5 — Failure mode:** host listing failures are silent empty list (no crash).

### README and docs

- [x] **P5.6 — Root README:** rewritten for real product.
- [x] **P5.7 — Privileges section:** sudo / setcap / stats fallback / `--strict-capture`.
- [x] **P5.8 — WSL2 section:** what works / limits.
- [x] **P5.9 — Build section:** Go 1.26.4, make build, demo/live/stats.

### Validation gates

- [x] **P5.10 — Unit:** `go test ./...` green.
- [x] **P5.11 — Vet/lint:** `go vet ./...` green.
- [x] **P5.12 — Race:** `go test -race ./internal/store/memory` green.
- [x] **P5.13 — Build:** `make build` → `bin/openwire`; demo path ready.
- [x] **P5.14 — Manual live (Linux):** proven via `make test-live` (Docker `golang:1.26.4` + `CAP_NET_RAW`/`CAP_NET_ADMIN` + `--net=host`). Unprivileged hosts skip `TestLinuxEngineLiveCapture`.
- [x] **P5.15 — Scope gate:** no firewall/DNS/Portmaster/Gin/React code.

**Acceptance criteria:** developer can `make build && ./bin/openwire start` (stats or live); `--demo` always works; `--strict-capture` fails clearly without privileges; `make test-live` proves AF_PACKET when Docker is available.
