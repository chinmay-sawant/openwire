# OpenWire — Future Phase 8: OpenWire Collector and Privileged Boundary

> **Parent:** [00-overview.md](00-overview.md)
> **Status:** intentionally deferred; only pursue if OpenWire becomes responsible for enforcement
> **Estimated effort:** future platform/security program

---

Only pursue this phase if OpenWire becomes responsible for enforcement rather than remaining a client/reporting layer.

- [ ] **P8.1 — Process boundary:** introduce `openwired` as a separately permissioned collector/service; keep `openwire tui` and React clients unprivileged.
- [ ] **P8.2 — Linux adapter:** isolate nftables/iptables, eBPF, process attribution, rollback, and capability handling behind explicit platform ports.
- [ ] **P8.3 — Windows adapter:** isolate WFP/driver, service ACL, signed artifacts, upgrade, and rollback behavior behind explicit platform ports.
- [ ] **P8.4 — Persistence:** choose backend-owned history storage and migrations; keep TUI preferences separate from operational data.
- [ ] **P8.5 — Security validation:** require privilege minimization, fail-closed behavior, crash/partial-start rollback, package tests, and platform-specific security review before enabling enforcement.
