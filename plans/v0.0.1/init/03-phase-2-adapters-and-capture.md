# OpenWire — Phase 2: Adapter Discovery and Packet Capture (Linux)

> **Parent:** [00-overview.md](00-overview.md)  
> **Status:** proposed; no implementation started  
> **Estimated effort:** one capture-engine slice

---

**Dependencies:** Phase 1  
**Milestone:** on Linux, OpenWire can list adapters and stream packet/byte observations into a channel without a TUI dependency.

## Scope

Wireshark-like **monitoring**: observe packets that appear on selected interfaces. v0.0.1 does **not** implement a full protocol dissector UI; it needs enough of each packet to:

- count bytes (rx/tx direction as best-effort)
- extract 5-tuple (src/dst IP, ports, protocol) when present
- feed the in-memory store and attribution layer

## Checklist

- [ ] **P2.1 — Adapter discovery (Linux):** implement `ListAdapters()` using netlink / `net.Interfaces` + addresses. Return name, index, hardware addr, IPv4/IPv6, flags (up, loopback, running).
- [ ] **P2.2 — Adapter selection:** default = all non-loopback up interfaces; honor `--iface`; always allow loopback if explicitly requested.
- [ ] **P2.3 — Capture engine port:** define a small interface, e.g.:

```go
type Engine interface {
    Start(ctx context.Context, ifaces []string) (<-chan Observation, <-chan error, error)
}
```

  `Observation` carries timestamp, iface, direction (if known), length, 5-tuple fields, raw optional metadata — **not** UI types.

- [ ] **P2.4 — Linux live capture:** implement via gopacket/pcap or AF_PACKET. Prefer a path that works on stock Linux with libpcap present; document build tags / CGO needs.
- [ ] **P2.5 — Privilege probe:** before open, check capability/root; return a typed error (`ErrInsufficientPrivilege`) with a human message for the TUI and CLI.
- [ ] **P2.6 — Demo capture:** implement `DemoEngine` that emits realistic synthetic observations on a ticker (varying apps later wired in Phase 3) so UI work is unblocked.
- [ ] **P2.7 — Shutdown:** cancel context stops sniffer goroutines; no leaked file descriptors; unit/integration test with short capture window under demo engine.
- [ ] **P2.8 — Direction / counters:** define best-effort rx vs tx (local address membership vs remote). Document uncertainty when NAT/WSL is in play.

**Required proof:** unit tests for adapter listing (table-driven with fakes); demo engine produces non-zero observations; live capture test is optional/skipped without privileges and documents skip reason.

**Acceptance criteria:** from a small CLI helper or test harness, running the capture engine yields a stream of observations for at least one Linux interface when privileges allow.
