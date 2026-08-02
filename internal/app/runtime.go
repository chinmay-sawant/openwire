// Package app wires capture, store, attribution, and the TUI.
package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/chinmay-sawant/openwire/internal/capture"
	"github.com/chinmay-sawant/openwire/internal/domain"
	"github.com/chinmay-sawant/openwire/internal/platform/paths"
	"github.com/chinmay-sawant/openwire/internal/platform/wsl"
	"github.com/chinmay-sawant/openwire/internal/store/memory"
	"github.com/chinmay-sawant/openwire/internal/store/sqlite"
	"github.com/chinmay-sawant/openwire/internal/tui"
)

// Config is the runtime configuration for `openwire start`.
type Config struct {
	Demo     bool
	Ifaces   []string
	Theme    string
	LogLevel string
	// StrictCapture disables unprivileged /proc stats fallback when AF_PACKET is unavailable.
	StrictCapture bool
	// DBPath is the SQLite file; empty + NoDB=false uses the default state-dir path.
	DBPath string
	// NoDB disables SQLite persistence entirely.
	NoDB bool
	// PCAPPath writes a Wireshark-compatible capture when non-empty (live AF_PACKET only).
	PCAPPath string
	// PCAPMaxMB rotates the pcap file after this many megabytes (0 = unbounded).
	PCAPMaxMB int
	// Deep prefers conntrack-based deep attribution when AF_PACKET is unavailable.
	Deep bool
	// EBPF enables optional eBPF kprobe counters (best-effort; requires privileges).
	EBPF bool
}

// Run starts capture (or demo) and the Bubble Tea UI. Blocks until quit.
func Run(ctx context.Context, cfg Config) error {
	store := memory.New(memory.Config{})
	isWSL := wsl.IsWSL2()

	var rec *sqlite.Recorder
	if !cfg.NoDB {
		dbPath := cfg.DBPath
		if dbPath == "" {
			p, err := paths.DefaultDBPath()
			if err != nil {
				slog.Warn("default db path", "err", err)
			} else {
				dbPath = p
			}
		}
		if dbPath != "" {
			r, err := sqlite.Open(dbPath)
			if err != nil {
				slog.Warn("sqlite open failed; continuing without persistence", "err", err, "path", dbPath)
			} else {
				rec = r
				defer rec.Close()
				if hist, err := rec.LoadRecentSamples(120); err != nil {
					slog.Warn("sqlite load samples", "err", err)
				} else if len(hist) > 0 {
					store.LoadSamples(hist)
					slog.Info("loaded historical samples", "n", len(hist), "db", dbPath)
				}
			}
		}
	}

	var (
		engine     capture.Engine
		mode       domain.CaptureMode
		attrs      *capture.Attributor
		ifaces     []string
		privOK     bool
		msg        string
		pcapWriter *capture.PCAPWriter
	)

	if cfg.Demo {
		mode = domain.ModeDemo
		engine = &capture.DemoEngine{Interval: 100 * time.Millisecond}
		adapters := capture.DemoAdapters()
		if isWSL {
			host := wsl.ListWindowsHostAdapters(ctx)
			adapters = append(adapters, host...)
		}
		store.SetAdapters(adapters)
		ifaces = capture.SelectIfaces(adapters, cfg.Ifaces)
		if len(ifaces) == 0 {
			ifaces = []string{"eth0"}
		}
		privOK = true
		msg = "demo mode"
		store.SetStatus(domain.Status{
			Mode: mode, Running: true, PrivilegeOK: privOK, Message: msg, IsWSL2: isWSL,
		})
	} else {
		linuxAdapters, err := capture.ListLinuxAdapters()
		if err != nil {
			return fmt.Errorf("list adapters: %w", err)
		}
		adapters := linuxAdapters
		if isWSL {
			host := wsl.ListWindowsHostAdapters(ctx)
			adapters = append(adapters, host...)
		}
		store.SetAdapters(adapters)
		ifaces = capture.SelectIfaces(linuxAdapters, cfg.Ifaces)
		if len(ifaces) == 0 {
			return domain.ErrNoAdapters
		}

		afErr := capture.CanOpenCapture(ifaces[0])
		switch {
		case afErr == nil:
			mode = domain.ModeLive
			le := &capture.LinuxEngine{}
			if cfg.PCAPPath != "" {
				max := int64(cfg.PCAPMaxMB) * 1024 * 1024
				if cfg.PCAPMaxMB <= 0 {
					max = 256 * 1024 * 1024
				}
				w, err := capture.NewPCAPWriter(cfg.PCAPPath, max)
				if err != nil {
					return fmt.Errorf("pcap: %w", err)
				}
				pcapWriter = w
				le.PCAP = w
				msg = "packet capture (AF_PACKET) · pcap " + cfg.PCAPPath
			} else {
				msg = "packet capture (AF_PACKET)"
			}
			engine = le
			attrs = capture.NewAttributor(2 * time.Second)
			privOK = true

		case cfg.Deep || (afErr != nil && capture.ConntrackAvailable() && !cfg.StrictCapture):
			// Deep: conntrack (+ optional eBPF) when raw capture is unavailable.
			if cfg.StrictCapture && !cfg.Deep {
				return afErr
			}
			mode = domain.ModeStats
			engine = &capture.DeepEngine{
				Interval: time.Second,
				UseEBPF:  cfg.EBPF,
				Attr:     capture.NewAttributor(2 * time.Second),
			}
			privOK = true
			msg = "deep attribution (conntrack"
			if cfg.EBPF {
				msg += "+ebpf"
			}
			msg += ")"
			if afErr != nil {
				slog.Warn("AF_PACKET unavailable; using deep attribution", "err", afErr)
			}

		case afErr != nil:
			if cfg.StrictCapture || !errors.Is(afErr, domain.ErrInsufficientPrivilege) {
				return afErr
			}
			mode = domain.ModeStats
			engine = &capture.ProcStatsEngine{Interval: time.Second}
			attrs = capture.NewAttributor(2 * time.Second)
			privOK = false
			msg = "stats mode (no CAP_NET_RAW): /proc counters, best-effort per-app"
			slog.Warn("falling back to unprivileged stats mode", "err", afErr)

		default:
			return fmt.Errorf("no capture path available")
		}

		store.SetStatus(domain.Status{
			Mode: mode, Running: true, PrivilegeOK: privOK, Message: msg, IsWSL2: isWSL,
		})
	}

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	if pcapWriter != nil {
		defer pcapWriter.Close()
	}

	obs, errCh, err := engine.Start(runCtx, ifaces)
	if err != nil {
		return err
	}

	if attrs != nil {
		go attrs.Run(runCtx.Done())
	}

	// Optional eBPF side-channel on live AF_PACKET path (does not replace packet capture).
	if cfg.EBPF && mode == domain.ModeLive && !cfg.Demo {
		go runEBPFSide(runCtx, store, ifaces)
		msg += " · ebpf side-counters"
		store.SetStatus(domain.Status{
			Mode: mode, Running: true, PrivilegeOK: privOK, Message: msg, IsWSL2: isWSL,
		})
	}

	if rec != nil {
		go rec.Run(runCtx, store)
		if msg != "" {
			msg += " · "
		}
		msg += "db " + rec.Path()
		store.SetStatus(domain.Status{
			Mode: mode, Running: true, PrivilegeOK: privOK, Message: msg, IsWSL2: isWSL,
		})
	}

	if isWSL && !cfg.Demo {
		go sampleHostLoop(runCtx, store, isWSL, mode, privOK, msg)
	}

	go func() {
		for {
			select {
			case <-runCtx.Done():
				return
			case err, ok := <-errCh:
				if !ok {
					errCh = nil
					continue
				}
				if err != nil {
					slog.Warn("capture error", "err", err)
					store.SetStatus(domain.Status{
						Mode:        mode,
						Running:     true,
						PrivilegeOK: privOK && !errors.Is(err, domain.ErrInsufficientPrivilege),
						Message:     err.Error(),
						IsWSL2:      isWSL,
					})
				}
			case o, ok := <-obs:
				if !ok {
					return
				}
				if attrs != nil {
					attrs.Annotate(&o)
				}
				// Prefer binary name over empty/unknown whenever we have a PID.
				if (o.AppHint == "" || o.AppHint == "unknown" || strings.HasPrefix(o.AppHint, "pid:")) && o.PIDHint > 0 {
					if n := capture.ProcessName(o.PIDHint); n != "" {
						o.AppHint = n
					}
				}
				store.Ingest(o)
			}
		}
	}()

	model := tui.NewModel(store, cfg.Theme)
	p := tea.NewProgram(
		model,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
		tea.WithContext(runCtx),
	)

	_, err = p.Run()
	cancel()
	return err
}

func runEBPFSide(ctx context.Context, store *memory.Store, ifaces []string) {
	c, err := capture.StartEBPFCounters(ctx)
	if err != nil {
		slog.Warn("ebpf side-counters unavailable", "err", err)
		return
	}
	defer c.Close()
	iface := "ebpf"
	if len(ifaces) > 0 {
		iface = ifaces[0]
	}
	prev := map[uint32]uint64{}
	t := time.NewTicker(time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-t.C:
			cur, err := c.Snapshot()
			if err != nil {
				continue
			}
			for pid, total := range cur {
				p := prev[pid]
				var d uint64
				if total >= p {
					d = total - p
				}
				prev[pid] = total
				if d == 0 {
					continue
				}
				name := capture.ProcessName(int(pid))
				if name == "" {
					name = fmt.Sprintf("pid:%d", pid)
				}
				// Event counts scaled so send-heavy apps appear when AF_PACKET is quiet.
				store.Ingest(domain.Observation{
					Time:      now,
					Iface:     iface,
					Direction: domain.DirectionTx,
					Length:    int(d * 512),
					Protocol:  domain.ProtoTCP,
					AppHint:   name,
					PIDHint:   int(pid),
				})
			}
		}
	}
}

func sampleHostLoop(ctx context.Context, store *memory.Store, isWSL bool, mode domain.CaptureMode, privOK bool, baseMsg string) {
	prev := map[string]wsl.HostAdapterStats{}
	// Warm process cache early so first traffic tick can attribute by name.
	go func() { _ = wsl.ListWindowsProcesses(ctx) }()

	if stats := wsl.SampleHostAdapterStats(ctx); len(stats) > 0 {
		adapters := store.ListAdapters()
		merged, next := wsl.MergeHostStatsIntoAdapters(adapters, stats, nil, 1)
		prev = next
		store.SetAdapters(merged)
	}

	t := time.NewTicker(2 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-t.C:
			stats := wsl.SampleHostAdapterStats(ctx)
			if len(stats) == 0 {
				continue
			}
			adapters := store.ListAdapters()
			merged, _ := wsl.MergeHostStatsIntoAdapters(adapters, stats, prev, 2.0)
			store.SetAdapters(merged)

			procs := wsl.ListWindowsProcesses(ctx)
			obs, next := wsl.HostTrafficObservationsByProcess(now, stats, prev, procs)
			prev = next
			for _, o := range obs {
				store.Ingest(o)
			}
			// Status shows whether process split worked (not aggregate-only).
			msg := baseMsg
			if len(procs) > 0 {
				named := 0
				for _, p := range procs {
					if p.Name != "" && !strings.HasPrefix(p.Name, "pid:") {
						named++
					}
				}
				msg = fmt.Sprintf("win %d apps", named)
			} else {
				msg = "win aggregate (no process list)"
				slog.Debug("windows process list empty; traffic attributed to windows-host")
			}
			store.SetStatus(domain.Status{
				Mode: mode, Running: true, PrivilegeOK: privOK, Message: msg, IsWSL2: isWSL,
			})
		}
	}
}
