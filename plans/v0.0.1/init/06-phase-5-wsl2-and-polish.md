# OpenWire — Phase 5: WSL2 Awareness, README, and Release Gates

> **Parent:** [00-overview.md](00-overview.md)  
> **Status:** mostly implemented (manual live capture proof optional on privileged hosts)  
> **Estimated effort:** one environment + documentation + validation slice

---

**Dependencies:** Phases 1–4  
**Milestone:** Linux path solid; WSL2 honest; README matches product; quality gates pass.

## Checklist

### WSL2 / Windows adapter visibility

- [x] **P5.1 — Environment detect:** `wsl.IsWSL2()`.
- [x] **P5.2 — Host adapter listing:** PowerShell / ipconfig best-effort.
- [x] **P5.3 — UI labeling:** `win:` prefix for windows-host adapters; WSL2 badge in header.
- [~] **P5.4 — Traffic path:** Linux AF_PACKET on WSL eth; host per-app attribution not available — documented. Next gate: optional Windows helper agent in a future version.
- [x] **P5.5 — Failure mode:** host listing failures are silent empty list (no crash).

### README and docs

- [x] **P5.6 — Root README:** rewritten for real product.
- [x] **P5.7 — Privileges section:** sudo / setcap.
- [x] **P5.8 — WSL2 section:** what works / limits.
- [x] **P5.9 — Build section:** Go 1.26.4, make build, demo/live.

### Validation gates

- [x] **P5.10 — Unit:** `go test ./...` green.
- [x] **P5.11 — Vet/lint:** `go vet ./...` green.
- [x] **P5.12 — Race:** `go test -race ./internal/store/memory` green.
- [x] **P5.13 — Build:** `make build` → `bin/openwire`; demo path ready.
- [~] **P5.14 — Manual live (Linux):** requires sudo/CAP_NET_RAW on a host with traffic — not asserted in CI. Operator should run `sudo ./bin/openwire start` once.
- [x] **P5.15 — Scope gate:** no firewall/DNS/Portmaster/Gin/React code.

**Acceptance criteria:** developer can `make build && ./bin/openwire start --demo`; live mode fails clearly without privileges.
