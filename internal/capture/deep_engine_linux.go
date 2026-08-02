//go:build linux

package capture

import (
	"context"
	"log/slog"
	"strconv"
	"time"

	"github.com/chinmay-sawant/openwire/internal/domain"
)

// DeepEngine attributes traffic using nf_conntrack byte counters + /proc process
// maps, optionally merged with eBPF per-pid counters when available.
// This is more accurate than /proc/net/dev proportional split (ProcStatsEngine).
type DeepEngine struct {
	Interval time.Duration
	// UseEBPF tries to attach an eBPF kprobe counter map (best-effort).
	UseEBPF bool
	// Attr is refreshed for process names; if nil a private one is used.
	Attr *Attributor
}

// Start implements Engine.
func (e *DeepEngine) Start(ctx context.Context, ifaces []string) (<-chan domain.Observation, <-chan error, error) {
	interval := e.Interval
	if interval <= 0 {
		interval = time.Second
	}
	iface := firstIface(ifaces)

	attr := e.Attr
	if attr == nil {
		attr = NewAttributor(2 * time.Second)
	}
	go attr.Run(ctx.Done())

	var ebpf *EBPFCounter
	if e.UseEBPF {
		c, err := StartEBPFCounters(ctx)
		if err != nil {
			slog.Warn("ebpf deep attribution unavailable", "err", err)
		} else {
			ebpf = c
			slog.Info("ebpf deep attribution active")
		}
	}

	// Require conntrack for this engine's primary signal.
	if _, err := readConntrack(); err != nil {
		if ebpf == nil {
			return nil, nil, err
		}
		// eBPF-only path still ok
	}

	obs := make(chan domain.Observation, 512)
	errs := make(chan error, 1)

	go func() {
		defer close(obs)
		defer close(errs)
		if ebpf != nil {
			defer ebpf.Close()
		}

		prevCT := map[string]uint64{}
		prevEBPF := map[uint32]uint64{}

		// Seed
		if entries, err := readConntrack(); err == nil {
			for _, e := range entries {
				prevCT[ctKey(e)] = e.bytes
			}
		}
		if ebpf != nil {
			if m, err := ebpf.Snapshot(); err == nil {
				prevEBPF = m
			}
		}

		t := time.NewTicker(interval)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case now := <-t.C:
				// Conntrack deltas → process via local endpoints.
				if entries, err := readConntrack(); err == nil {
					for _, ent := range entries {
						k := ctKey(ent)
						prev := prevCT[k]
						var delta uint64
						if ent.bytes >= prev {
							delta = ent.bytes - prev
						}
						prevCT[k] = ent.bytes
						if delta == 0 {
							continue
						}
						o := domain.Observation{
							Time:      now,
							Iface:     iface,
							Direction: domain.DirectionUnknown,
							Length:    clampLen(delta),
							Protocol:  ent.proto,
							SrcIP:     ent.src,
							DstIP:     ent.dst,
							SrcPort:   ent.sport,
							DstPort:   ent.dport,
						}
						// Heuristic direction: if src is local socket, treat as tx.
						attr.Annotate(&o)
						// Prefer local endpoint for annotation: try both sides already in Annotate.
						if o.AppHint == "" {
							// reverse annotate attempt with swapped tuple
							rev := o
							rev.SrcIP, rev.DstIP = o.DstIP, o.SrcIP
							rev.SrcPort, rev.DstPort = o.DstPort, o.SrcPort
							attr.Annotate(&rev)
							if rev.AppHint != "" {
								o.AppHint = rev.AppHint
								o.PIDHint = rev.PIDHint
							}
						}
						if o.AppHint == "" {
							o.AppHint = "unknown"
						}
						// Direction guess: if we matched src local → tx else rx
						if o.PIDHint > 0 {
							// leave unknown; store still counts
						}
						select {
						case obs <- o:
						case <-ctx.Done():
							return
						default:
						}
					}
				} else {
					select {
					case errs <- err:
					default:
					}
				}

				// eBPF per-pid event deltas (scale as small synthetic bytes so UI shows activity).
				if ebpf != nil {
					cur, err := ebpf.Snapshot()
					if err == nil {
						for pid, total := range cur {
							prev := prevEBPF[pid]
							var delta uint64
							if total >= prev {
								delta = total - prev
							}
							prevEBPF[pid] = total
							if delta == 0 {
								continue
							}
							// Each send event counted as ~1 KiB placeholder when conntrack missed it.
							name, _ := procName(int(pid))
							o := domain.Observation{
								Time:      now,
								Iface:     iface,
								Direction: domain.DirectionTx,
								Length:    clampLen(delta * 1024),
								Protocol:  domain.ProtoTCP,
								AppHint:   name,
								PIDHint:   int(pid),
							}
							select {
							case obs <- o:
							case <-ctx.Done():
								return
							default:
							}
						}
					}
				}
			}
		}
	}()

	return obs, errs, nil
}

func ctKey(e connEntry) string {
	return string(e.proto) + "|" + e.src + ":" + itoaPort(e.sport) + "-" + e.dst + ":" + itoaPort(e.dport)
}

func itoaPort(p uint16) string {
	return strconv.FormatUint(uint64(p), 10)
}

func clampLen(n uint64) int {
	const max = 1 << 28
	if n > max {
		return max
	}
	return int(n)
}
