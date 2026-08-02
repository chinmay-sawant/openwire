# OpenWire — Phase 4: Bubble Tea TUI (Dark, Mouse, Graphs)

> **Parent:** [00-overview.md](00-overview.md)  
> **Status:** implemented  
> **Estimated effort:** one UI vertical slice

---

**Dependencies:** Phase 3  
**Milestone:** `openwire start` shows graph + app list, navigable with mouse and keyboard, dark theme default.

## Checklist

- [x] **P4.1 — Dark theme default:** Lip Gloss palette in `internal/tui/theme.go`.
- [x] **P4.2 — Program options:** `tea.WithMouseCellMotion()`, `WithAltScreen`.
- [x] **P4.3 — Graph pane:** sparkline from BandwidthSample ring.
- [x] **P4.4 — App list pane:** sorted by rate; bars + up/down totals.
- [x] **P4.5 — Keyboard navigation:** arrows/jk, tab, enter, esc, q.
- [x] **P4.6 — Mouse navigation:** click rows/panes, scroll wheel.
- [x] **P4.7 — Status / adapters bar:** mode, adapters, rates, WSL2 flag, messages.
- [x] **P4.8 — Resize handling:** WindowSizeMsg; too-small message.
- [x] **P4.9 — Live wiring:** TUI reads store only via `internal/app`.
- [x] **P4.10 — Demo polish:** `openwire start --demo` sufficient for UX without root.

**Required proof:** binary runs demo TUI; format helper tests.
