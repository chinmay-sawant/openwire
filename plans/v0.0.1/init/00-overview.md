# OpenWire v0.0.1 — Network Traffic Monitor (TUI)

> **Parent:** none — initial product and architecture plan  
> **Status:** proposed; no implementation started  
> **Estimated effort:** 6 dependency-ordered phases to a one-command usable monitor

---

## Plan Index

This directory is the canonical plan for the first OpenWire release. Each phase file is a live checklist; update rows only with evidence.

| File | Phase |
|------|--------|
| [01-phase-0-product-goals.md](01-phase-0-product-goals.md) | Product goals and runtime contract |
| [02-phase-1-go-cli-skeleton.md](02-phase-1-go-cli-skeleton.md) | Go 1.26.4 module + `openwire start` |
| [03-phase-2-adapters-and-capture.md](03-phase-2-adapters-and-capture.md) | Linux adapters + packet capture |
| [04-phase-3-store-and-attribution.md](04-phase-3-store-and-attribution.md) | In-memory store + per-app bandwidth |
| [05-phase-4-bubbletea-tui.md](05-phase-4-bubbletea-tui.md) | Bubble Tea UI (dark, mouse + keys, graphs) |
| [06-phase-5-wsl2-and-polish.md](06-phase-5-wsl2-and-polish.md) | WSL2/Windows adapter awareness + README + gates |
| [07-dependencies-non-goals.md](07-dependencies-non-goals.md) | Dependencies, non-goals, open decisions |

## Product Goal (one sentence)

**Detect local network traffic → keep it in memory → show which applications use how much bandwidth in a GlassWire-style Bubble Tea UI, started with a single command.**

## What v0.0.1 Delivers

```text
openwire start
```

1. Discovers local network adapters (Linux first).
2. Captures traffic on selected/all adapters (Wireshark-style monitor of packets that pass the interface).
3. Stores flows/bytes in an **in-memory** ring/store (SQLite or similar is explicitly later).
4. Attributes usage to applications/processes where possible.
5. Opens a **Bubble Tea** terminal UI:
   - **Dark theme by default**
   - **Mouse** and **arrow-key** navigation
   - Live bandwidth graph (GlassWire-like) + app list sorted by usage
6. When run under **WSL2**, detects and reports Windows host adapters / host-side traffic visibility as far as the environment allows (see Phase 5).

## Explicit Stack Decisions

| Decision | Choice |
|----------|--------|
| Language / toolchain | **Go 1.26.4** (`go.mod` toolchain pin) |
| CLI entry | **`openwire start`** (single primary command) |
| TUI | **Bubble Tea** + Bubbles + Lip Gloss |
| Theme | **Dark by default** |
| Input | Keyboard (arrows, tab, enter, q) **and mouse** (click, scroll, focus) |
| Capture platform (v0.0.1) | **Linux** first (`AF_PACKET` / libpcap via gopacket or equivalent) |
| Storage (v0.0.1) | **In-memory only** (bounded rings / maps) |
| Persistence later | SQLite (or similar) — not in this plan’s delivery bar |
| UI inspiration | **GlassWire**: per-app bandwidth + time-series graph in the terminal |
| Capture inspiration | **Wireshark**: observe traffic on interfaces; OpenWire does **not** ship a full packet-dissector UI in v0.0.1 |

## Target Architecture

```text
cmd/openwire
    └── start
            │
            ▼
    application runtime (context, lifecycle)
            │
    ┌───────┼───────────────────────────┐
    ▼       ▼                           ▼
 adapter  capture                    store
 discovery engine                   (memory)
 (linux)  (linux/wsl)                   │
    │       │                           ▼
    │       └────────────► attribution (pid / process / app)
    │                                   │
    └───────────────────────────────────┤
                                        ▼
                              Bubble Tea TUI (dark)
                              - graph pane
                              - app bandwidth list
                              - adapter / status bar
                              - mouse + keyboard nav
```

**Boundary rules**

- Domain/store types do **not** import Bubble Tea or tcell.
- Capture code is isolated under platform packages (`internal/capture/linux`, …).
- TUI only consumes read models / event streams from the application layer.
- Privilege requirements (e.g. `CAP_NET_RAW` / root for live capture) are explicit in the README and surfaced in the UI when missing.

## Runtime Model

| Mode | Command | Behavior |
|------|---------|----------|
| Live (default) | `openwire start` | Discover adapters, capture traffic, fill in-memory store, show TUI |
| Demo / no-priv | `openwire start --demo` | Deterministic fake traffic so UI/layout can be developed without capture privileges |
| Interface pin | `openwire start --iface eth0` | Capture only named interface(s) |

Foreground process only for v0.0.1: no daemon, no system service installer, no tray app.

## Success Criteria for v0.0.1

- [ ] Built and run with **Go 1.26.4**.
- [ ] `openwire start` launches the dark Bubble Tea UI in one step.
- [ ] Mouse clicks and arrow keys can move focus and select apps/rows.
- [ ] On Linux, adapters are listed and live traffic increments per-app (or process) counters when capture privileges are available.
- [ ] Without privileges, the app fails with a clear message **or** falls back only when `--demo` is requested (no silent fake data in live mode).
- [ ] Traffic is held in a bounded in-memory store (no disk DB yet).
- [ ] Default home view is “apps by bandwidth” + live graph.
- [ ] README documents install, privileges, WSL2 notes, and limits.
- [ ] WSL2 path documents/implements host-adapter visibility strategy (Phase 5).

## Status Rules

- `[ ]` not started or not proven  
- `[x]` implemented and validated with current evidence  
- `[~]` deferred/partial — reason, boundary, next gate required  

Documentation-only plan edits do not close implementation rows and do not require lint/test.

## What This Plan Replaces

The previous Portmaster-client / Gin-React / policy-control plan is **withdrawn**. OpenWire v0.0.1 is a **local traffic monitor with a TUI**, not a Portmaster UI clone and not a firewall controller.
