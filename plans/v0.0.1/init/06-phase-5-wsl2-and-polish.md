# OpenWire — Phase 5: WSL2 Awareness, README, and Release Gates

> **Parent:** [00-overview.md](00-overview.md)  
> **Status:** proposed; no implementation started  
> **Estimated effort:** one environment + documentation + validation slice

---

**Dependencies:** Phases 1–4  
**Milestone:** Linux path is solid; WSL2 behavior is honest and as capable as the environment allows; README matches the product; quality gates pass.

## WSL2 reality (plan assumptions)

Under WSL2:

- Linux-side adapters (e.g. `eth0`) usually see **NAT’d** traffic for Linux processes.
- **Windows applications’** traffic does not automatically appear as Linux process attribution.
- “Detect Windows adapters” requires an explicit strategy, not wishful binding to `GetAdaptersAddresses` from inside a pure Linux binary.

### v0.0.1 WSL2 strategy (chosen)

1. **Detect WSL2** (`/proc/version`, `WSL_INTEROP`, `WSL_DISTRO_NAME`, etc.).
2. **Surface Windows host network adapters** via a small, documented path:
   - Prefer calling into Windows from WSL when available, e.g. `powershell.exe` / `ipconfig.exe` / `Get-NetAdapter` for **adapter inventory** (names, status, host IPs).
   - Show these adapters in the TUI as **host adapters** (clearly labeled), separate from Linux interfaces.
3. **Traffic for Windows apps:**
   - Best-effort: if mirrored networking / experimental features expose host traffic on a Linux-visible interface, capture it and label sources carefully.
   - If host process attribution is not available from Linux, show host adapter stats and aggregate host-side counters when obtainable (e.g. performance counters via PowerShell) **without** pretending they are Linux PIDs.
4. Never silently claim Windows Chrome is a Linux process.

If a richer host capture agent is needed later, that is a **future** Windows helper — not a blocker that reintroduces Portmaster complexity into v0.0.1.

## Checklist

### WSL2 / Windows adapter visibility

- [ ] **P5.1 — Environment detect:** `IsWSL2()` helper with tests for common markers.
- [ ] **P5.2 — Host adapter listing:** when WSL2 is detected, list Windows adapters (name, status, IPv4) into the store/UI as `Adapter{Source: "windows-host", ...}`.
- [ ] **P5.3 — UI labeling:** adapters and status bar distinguish `linux` vs `windows-host`; help text explains limits of process attribution under WSL2.
- [ ] **P5.4 — Traffic path:** document and implement the best available host traffic signal for v0.0.1 (Linux iface capture and/or host counters). Mark incomplete pieces `[~]` with exact next gate rather than faking per-app Windows data.
- [ ] **P5.5 — Failure mode:** if `powershell.exe` is unavailable, show a clear “host adapters unavailable” state without crashing.

### README and docs

- [ ] **P5.6 — Root README:** rewrite `README.md` for the real product (see outline below).
- [ ] **P5.7 — Privileges section:** how to run with capabilities (`setcap`) or sudo; why capture needs them.
- [ ] **P5.8 — WSL2 section:** what works, what does not, how host adapters appear.
- [ ] **P5.9 — Build section:** require Go **1.26.4**, `make build`, `openwire start`, `openwire start --demo`.

### Validation gates

- [ ] **P5.10 — Unit:** `make test` (or `go test ./...`) green.
- [ ] **P5.11 — Vet/lint:** `go vet ./...` and agreed linter green.
- [ ] **P5.12 — Race:** `go test -race` on store, capture demo, and TUI update paths that are unit-testable.
- [ ] **P5.13 — Build:** release-style binary for linux/amd64; smoke `openwire start --demo`.
- [ ] **P5.14 — Manual live (Linux):** with privileges, verify adapters + at least one real app’s counters move while generating traffic.
- [ ] **P5.15 — Scope gate:** confirm no firewall/DNS/Portmaster/Gin/React code was reintroduced.

## README outline (required content)

```markdown
# OpenWire
One-line: terminal network monitor (GlassWire-like app usage + Wireshark-like capture).

## Requirements
- Go 1.26.4
- Linux (primary); WSL2 notes
- libpcap / privileges for live capture

## Quick start
make build
./bin/openwire start --demo
sudo ./bin/openwire start

## Features (v0.0.1)
- Adapter discovery
- Live capture → in-memory store
- Per-app bandwidth
- Bubble Tea UI, dark theme, mouse + keyboard

## WSL2
...

## Non-goals
- Firewall, DNS filter, SQLite (yet), desktop GUI
```

**Acceptance criteria:** a developer on Linux can clone, build with Go 1.26.4, run demo without root, run live with privileges and see traffic; a developer on WSL2 can see host adapters listed and understands attribution limits from the README.
