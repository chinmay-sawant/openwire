# OpenWire — Phase 2: Adapter Discovery and Packet Capture (Linux)

> **Parent:** [00-overview.md](00-overview.md)  
> **Status:** implemented  
> **Estimated effort:** one capture-engine slice

---

**Dependencies:** Phase 1  
**Milestone:** on Linux, OpenWire can list adapters and stream packet/byte observations into a channel without a TUI dependency.

## Checklist

- [x] **P2.1 — Adapter discovery (Linux):** `ListLinuxAdapters()` via `net.Interfaces`.
- [x] **P2.2 — Adapter selection:** non-loopback up by default; honor `--iface`.
- [x] **P2.3 — Capture engine port:** `capture.Engine` with `Start(ctx, ifaces)`.
- [x] **P2.4 — Linux live capture:** pure-Go **AF_PACKET** (no libpcap/CGO) in `linux_afpacket.go`.
- [x] **P2.5 — Privilege probe:** `CanOpenCapture` → `ErrInsufficientPrivilege`.
- [x] **P2.6 — Demo capture:** `DemoEngine` synthetic multi-app stream.
- [x] **P2.7 — Shutdown:** context cancel closes sockets / stops demo ticker.
- [x] **P2.8 — Direction / counters:** local-IP membership for rx/tx guess.

**Required proof:** adapter + demo tests pass; live mode without priv returns explicit error.
