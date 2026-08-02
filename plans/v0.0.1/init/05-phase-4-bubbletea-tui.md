# OpenWire — Phase 4: Bubble Tea TUI (Dark, Mouse, Graphs)

> **Parent:** [00-overview.md](00-overview.md)  
> **Status:** proposed; no implementation started  
> **Estimated effort:** one UI vertical slice (primary user-visible milestone)

---

**Dependencies:** Phase 3 (store queries); Phase 1 shell  
**Milestone:** `openwire start` (live or `--demo`) shows a GlassWire-inspired layout: live graph + app bandwidth list, navigable with mouse and keyboard, dark theme default.

## Layout (target)

```text
┌ OpenWire ──────────────────────────────── adapters: eth0 · wlan0 ─── live ─┐
│  Bandwidth (last 60s)                                                      │
│  ▂▃▅▇█▇▅▃▂▁▂▃▅▆█  total ▲ 1.2 MB/s  ▼ 340 KB/s                             │
├────────────────────────────────────────────────────────────────────────────┤
│  Applications (by usage)                          Focus: apps              │
│  ▶ browser          ████████████░░  820 KB/s   ↑410 ↓410                   │
│    code             ██████░░░░░░░░  210 KB/s   ↑180 ↓30                    │
│    unknown          ██░░░░░░░░░░░░   40 KB/s   ↑20  ↓20                    │
├────────────────────────────────────────────────────────────────────────────┤
│  Detail: browser · pid 1234 · /usr/bin/…                                   │
│  flows: 12 open · top remote: 1.2.3.4:443                                  │
├────────────────────────────────────────────────────────────────────────────┤
│  ↑↓/mouse select · tab panes · enter detail · q quit · dark theme          │
└────────────────────────────────────────────────────────────────────────────┘
```

Exact glyph/chart library is an implementation choice (Lip Gloss styles + braille/block sparklines). Look and feel: **GlassWire monitor** — apps + traffic over time — not a Wireshark packet list.

## Checklist

- [ ] **P4.1 — Dark theme default:** Lip Gloss palette for background, borders, accents, positive/negative rates; `--theme dark` is default; optional light later is fine but not required.
- [ ] **P4.2 — Program options:** enable mouse cell motion / mouse support so clicks and scroll work in capable terminals.
- [ ] **P4.3 — Graph pane:** render total (and optional selected-app) bandwidth history from `BandwidthSample` ring. Update on a tick (e.g. 500ms–1s) without blocking capture.
- [ ] **P4.4 — App list pane:** sorted by current rate or session bytes (toggle allowed); show name, rate bars, up/down. Default sort = highest consumers first.
- [ ] **P4.5 — Keyboard navigation:**
  - arrows / j k — move selection
  - tab — cycle panes (graph / apps / detail)
  - enter — open detail for selected app
  - esc — back
  - q / ctrl+c — quit cleanly
- [ ] **P4.6 — Mouse navigation:**
  - click row to select
  - click pane to focus
  - scroll wheel on list
  - no reliance on mouse-only actions (keyboard always works)
- [ ] **P4.7 — Status / adapters bar:** show discovered adapters, capture state (live / demo / error), privilege warning when capture failed.
- [ ] **P4.8 — Resize handling:** WindowSizeMsg reflows layout; minimum size message when terminal is too small.
- [ ] **P4.9 — Live wiring:** TUI reads only store snapshot/query APIs (or a thin app service). No direct pcap handles in view code.
- [ ] **P4.10 — Demo polish:** `openwire start --demo` alone is enough for screenshots and manual UX review without root.

**Required proof:** manual checklist recorded in phase closure notes; optional golden-string tests for pure render helpers; process always restores terminal on quit.

**Acceptance criteria:** a new user runs `openwire start --demo`, sees a dark UI with a moving graph and an app list, can move with arrows and click with the mouse, and quit with `q`.
