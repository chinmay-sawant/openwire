//go:build linux

package capture

import (
	"bufio"
	"context"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/chinmay-sawant/openwire/internal/domain"
)

// ProcStatsEngine samples /proc/net/dev and open sockets without CAP_NET_RAW.
// Interface byte deltas are attributed across processes that hold sockets,
// proportionally to each process's connection count (best-effort, not packet-accurate).
type ProcStatsEngine struct {
	Interval time.Duration
	// Attr is optional; when set, used only to keep process names fresh via Run elsewhere.
	// Socket→process mapping is refreshed inside this engine each tick.
}

type ifaceCounters struct {
	rx, tx uint64
}

type sockEndpoint struct {
	proto domain.Protocol
	ip    string
	port  uint16
	pid   int
	name  string
}

// Start implements Engine.
func (e *ProcStatsEngine) Start(ctx context.Context, ifaces []string) (<-chan domain.Observation, <-chan error, error) {
	interval := e.Interval
	if interval <= 0 {
		interval = time.Second
	}
	want := map[string]struct{}{}
	for _, n := range ifaces {
		want[n] = struct{}{}
	}

	obs := make(chan domain.Observation, 512)
	errs := make(chan error, 1)

	go func() {
		defer close(obs)
		defer close(errs)

		prev, err := readProcNetDev()
		if err != nil {
			select {
			case errs <- err:
			default:
			}
			return
		}
		// Seed prev so first tick is a delta after interval.
		t := time.NewTicker(interval)
		defer t.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case now := <-t.C:
				cur, err := readProcNetDev()
				if err != nil {
					select {
					case errs <- err:
					default:
					}
					continue
				}
				socks := listLocalSockets()
				// Group sockets by pid for attribution weights.
				type weight struct {
					pid   int
					name  string
					count int
					// representative endpoints
					eps []sockEndpoint
				}
				byPID := map[int]*weight{}
				for _, s := range socks {
					w, ok := byPID[s.pid]
					if !ok {
						w = &weight{pid: s.pid, name: s.name}
						byPID[s.pid] = w
					}
					w.count++
					if len(w.eps) < 4 {
						w.eps = append(w.eps, s)
					}
				}
				totalConns := 0
				for _, w := range byPID {
					totalConns += w.count
				}

				var totalRxDelta, totalTxDelta uint64
				for name, c := range cur {
					if len(want) > 0 {
						if _, ok := want[name]; !ok {
							continue
						}
					}
					p, ok := prev[name]
					if !ok {
						continue
					}
					var drx, dtx uint64
					if c.rx >= p.rx {
						drx = c.rx - p.rx
					}
					if c.tx >= p.tx {
						dtx = c.tx - p.tx
					}
					totalRxDelta += drx
					totalTxDelta += dtx
				}
				prev = cur

				if totalRxDelta == 0 && totalTxDelta == 0 {
					// Still emit tiny presence observations so UI shows active apps with open sockets.
					for _, w := range byPID {
						if w.pid <= 0 {
							continue
						}
						o := domain.Observation{
							Time:      now,
							Iface:     firstIface(ifaces),
							Direction: domain.DirectionUnknown,
							Length:    0,
							Protocol:  domain.ProtoTCP,
							AppHint:   w.name,
							PIDHint:   w.pid,
						}
						if len(w.eps) > 0 {
							o.Protocol = w.eps[0].proto
							o.SrcIP = w.eps[0].ip
							o.SrcPort = w.eps[0].port
						}
						// skip zero-length — store ignores usefulness; use 1 byte keepalive per app
						o.Length = 1
						o.Direction = domain.DirectionRx
						select {
						case obs <- o:
						case <-ctx.Done():
							return
						default:
						}
					}
					continue
				}

				if totalConns == 0 {
					// Attribute all to unknown.
					emitSplit(obs, ctx, now, firstIface(ifaces), totalRxDelta, totalTxDelta, "unknown", 0, nil)
					continue
				}

				// Distribute deltas by connection count.
				var assignedRx, assignedTx uint64
				i := 0
				n := len(byPID)
				for _, w := range byPID {
					i++
					var rxShare, txShare uint64
					if i < n {
						rxShare = totalRxDelta * uint64(w.count) / uint64(totalConns)
						txShare = totalTxDelta * uint64(w.count) / uint64(totalConns)
						assignedRx += rxShare
						assignedTx += txShare
					} else {
						// last pid gets remainder to avoid truncation loss
						rxShare = totalRxDelta - assignedRx
						txShare = totalTxDelta - assignedTx
					}
					emitSplit(obs, ctx, now, firstIface(ifaces), rxShare, txShare, w.name, w.pid, w.eps)
				}
			}
		}
	}()

	return obs, errs, nil
}

func emitSplit(obs chan<- domain.Observation, ctx context.Context, now time.Time, iface string, rx, tx uint64, name string, pid int, eps []sockEndpoint) {
	// Chunk large deltas into multiple observations so UI/graph stays responsive.
	const chunk = 64 * 1024
	send := func(dir domain.Direction, n uint64, ep *sockEndpoint) {
		for n > 0 {
			c := n
			if c > chunk {
				c = chunk
			}
			n -= c
			o := domain.Observation{
				Time:      now,
				Iface:     iface,
				Direction: dir,
				Length:    int(c),
				Protocol:  domain.ProtoOther,
				AppHint:   name,
				PIDHint:   pid,
			}
			if ep != nil {
				o.Protocol = ep.proto
				o.SrcIP = ep.ip
				o.SrcPort = ep.port
			}
			select {
			case obs <- o:
			case <-ctx.Done():
				return
			default:
				return
			}
		}
	}
	var ep *sockEndpoint
	if len(eps) > 0 {
		ep = &eps[0]
	}
	if rx > 0 {
		send(domain.DirectionRx, rx, ep)
	}
	if tx > 0 {
		send(domain.DirectionTx, tx, ep)
	}
}

func firstIface(ifaces []string) string {
	if len(ifaces) > 0 {
		return ifaces[0]
	}
	return "eth0"
}

func readProcNetDev() (map[string]ifaceCounters, error) {
	f, err := os.Open("/proc/net/dev")
	if err != nil {
		return nil, err
	}
	defer f.Close()
	out := make(map[string]ifaceCounters)
	sc := bufio.NewScanner(f)
	// skip 2 header lines
	for i := 0; i < 2 && sc.Scan(); i++ {
	}
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		// name: rx_bytes rx_packets ... tx_bytes ...
		col := strings.IndexByte(line, ':')
		if col < 0 {
			continue
		}
		name := strings.TrimSpace(line[:col])
		fields := strings.Fields(line[col+1:])
		if len(fields) < 9 {
			continue
		}
		rx, _ := strconv.ParseUint(fields[0], 10, 64)
		tx, _ := strconv.ParseUint(fields[8], 10, 64)
		out[name] = ifaceCounters{rx: rx, tx: tx}
	}
	return out, sc.Err()
}

// listLocalSockets returns local endpoints with owning pid/name.
func listLocalSockets() []sockEndpoint {
	inodeToPID := mapInodeToPID()
	var out []sockEndpoint
	for _, table := range []string{"/proc/net/tcp", "/proc/net/tcp6", "/proc/net/udp", "/proc/net/udp6"} {
		proto := domain.ProtoTCP
		if strings.Contains(table, "udp") {
			proto = domain.ProtoUDP
		}
		f, err := os.Open(table)
		if err != nil {
			continue
		}
		sc := bufio.NewScanner(f)
		if sc.Scan() {
			// header
		}
		for sc.Scan() {
			fields := strings.Fields(sc.Text())
			if len(fields) < 10 {
				continue
			}
			// skip listen sockets with remote 0 for tcp? keep all — weights still useful
			ip, port, ok := parseProcAddr(fields[1])
			if !ok {
				continue
			}
			pid, ok := inodeToPID[fields[9]]
			if !ok || pid <= 0 {
				continue
			}
			name, _ := procName(pid)
			out = append(out, sockEndpoint{
				proto: proto,
				ip:    ip,
				port:  port,
				pid:   pid,
				name:  name,
			})
		}
		_ = f.Close()
	}
	return out
}
