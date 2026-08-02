# OpenWire — Phase 0: Product and Runtime Contract

> **Parent:** [00-overview.md](00-overview.md)
> **Status:** proposed; no implementation started
> **Estimated effort:** one product/architecture decision slice

---

**Dependencies:** none  
**Milestone:** the first executable behavior and ownership boundary are unambiguous.

- [ ] **P0.1 — Runtime decision:** document `openwire tui --demo` and `openwire tui --endpoint URL` in `docs/runtime.md`, including foreground execution, signal handling, and no-elevation behavior.
- [ ] **P0.2 — Source selection:** define explicit `demo`, `portmaster`, and future `openwire` source modes; reject ambiguous or missing live-source configuration with actionable errors.
- [ ] **P0.3 — Configuration precedence:** define flags over environment variables over a user config file; identify the exact endpoint, credential, refresh, color, and log settings.
- [ ] **P0.4 — Scope boundary:** record that packet interception, firewall/DNS enforcement, process attribution, eBPF/nfqueue/WFP, SPN routing, installers, service management, kernel modules, auto-update, and React UI are out of scope for the first TUI.
- [ ] **P0.5 — Platform target:** choose the initial supported terminal/OS matrix. Default proposal is Linux first, with portability kept in pure Go and platform adapters isolated.
- [ ] **P0.6 — TUI framework:** choose Bubble Tea/Bubbles/Lip Gloss or an equivalent framework, define the minimum terminal size, color fallback, and raw-terminal restoration policy.
- [ ] **P0.7 — Read-only policy:** approve read-only behavior for the first vertical slice; list every later mutation that will require capability discovery, confirmation, post-save refresh, and audit logging.
- [ ] **P0.8 — Data contract:** define initial fields and stable identifiers for `Connection`, `ApplicationProfile`, `SystemStatus`, `Notification`, and `Report` without copying raw backend records into the UI.
- [ ] **P0.9 — Error contract:** define user-visible states for loading, unavailable, unauthorized, malformed data, stale data, reconnecting, terminal-too-small, and unsupported capability.

**Acceptance criteria:** `docs/runtime.md` answers how the application starts, what process it connects to, what privileges it requires, what happens when no backend is available, and what it explicitly does not do.
