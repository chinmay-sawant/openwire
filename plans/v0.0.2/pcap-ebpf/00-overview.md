# OpenWire v0.0.2 — PCAP Archive + Deep / eBPF Attribution

> **Status:** implemented  
> **Goal:** Wireshark-compatible packet archive + deeper Linux attribution

## PCAP archive

- Classic libpcap file format (linktype Ethernet)
- Tee from AF_PACKET live capture (`--pcap PATH`, `--pcap-max-mb N`)
- Rotates to `PATH.1` when size exceeded

## Deep attribution

- **Conntrack engine** (`DeepEngine`): `/proc/net/nf_conntrack` byte counters + `/proc` process maps
- Used when AF_PACKET is unavailable (or `--deep`) if conntrack is readable
- More accurate than proportional `/proc/net/dev` split

## eBPF (best-effort)

- Optional kprobe on `tcp_sendmsg` counting send events per PID (`--ebpf`)
- Requires privileges + tracefs; fails soft and continues without eBPF
- On live AF_PACKET path, runs as a side-channel; on deep path, merged into DeepEngine

## Flags

```text
--pcap PATH
--pcap-max-mb N   (default 256)
--deep
--ebpf
```

## Checklist

- [x] PCAP writer + tests
- [x] AF_PACKET tee
- [x] Conntrack parser + DeepEngine
- [x] eBPF kprobe counter (amd64, best-effort)
- [x] CLI + runtime wiring
- [x] Docs
