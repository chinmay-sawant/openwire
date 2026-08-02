# OpenWire — Dependencies, Non-Goals, and Open Decisions

> **Parent:** [00-overview.md](00-overview.md)  
> **Status:** proposed; no implementation started  
> **Estimated effort:** maintained alongside phase execution

---

## Dependency Order

```text
Phase 0  Product goals / runtime contract
   │
Phase 1  Go 1.26.4 module + openwire start + dark TUI shell
   │
Phase 2  Linux adapter discovery + capture engine (+ demo engine)
   │
Phase 3  In-memory store + process attribution + queries
   │
Phase 4  Bubble Tea full UI (graph, apps, mouse + keys)
   │
Phase 5  WSL2 host adapters + README + validation gates
```

No parallel “Portmaster adapter” or “Gin/React” track in v0.0.1.

## Suggested Libraries (minimal)

| Area | Suggestion | Notes |
|------|------------|--------|
| CLI | `spf13/cobra` or stdlib | Keep surface small |
| TUI | `charmbracelet/bubbletea`, `bubbles`, `lipgloss` | Dark theme via Lip Gloss |
| Capture | `gopacket` + libpcap **or** pure Go AF_PACKET | Document CGO if used |
| Tests | stdlib `testing` | race tests on store |

Pin versions in `go.mod` at implementation time; do not pre-add unused deps.

## Explicit Non-Goals (v0.0.1)

- Portmaster HTTP/WebSocket client or embedding Portmaster as a required backend  
- Firewall / allow-block policy engine  
- DNS interception or secure-DNS ownership  
- Full Wireshark-style packet decode UI (hex, protocol trees)  
- SQLite / disk persistence (interface may allow it later)  
- Gin API + React dashboard  
- System service / tray / auto-update / installer  
- Native Windows or macOS capture engines (WSL2 host **listing** is in scope; full native Windows agent is not)  
- eBPF-based deep attribution (optional future; `/proc` is enough for v0.0.1)  

## Open Decisions (resolve during the named phase, not ad hoc)

| # | Decision | Default if unspecified | Resolve in |
|---|----------|------------------------|------------|
| 1 | Module path | `github.com/chinmay/openwire` or repo-remote path | Phase 1 |
| 2 | CGO vs pure Go capture | Prefer whatever works reliably on Linux; document | Phase 2 |
| 3 | Default sort (rate vs total bytes) | Current rate | Phase 4 |
| 4 | Sample interval / ring length | 1s samples, 60–120 points | Phase 3 |
| 5 | WSL2 host traffic depth | Adapter list + best-effort counters; honest limits | Phase 5 |
| 6 | Log destination with TUI active | File under `$XDG_STATE_HOME/openwire/` or `/tmp` | Phase 1 |

## Withdrawn Plan Material

Everything in the old plan set that targeted:

- `openwire tui --endpoint` Portmaster client  
- read-only Portmaster adapter  
- safe policy controls  
- Gin + React reporting  
- privileged `openwired` collector as the first product  

…is **not** part of v0.0.1. Revisit only in a future version plan under `plans/v0.x.y/…`.
