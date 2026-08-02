// Package app wires capture, store, attribution, and the TUI.
package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/chinmay-sawant/openwire/internal/capture"
	"github.com/chinmay-sawant/openwire/internal/domain"
	"github.com/chinmay-sawant/openwire/internal/platform/wsl"
	"github.com/chinmay-sawant/openwire/internal/store/memory"
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
}

// Run starts capture (or demo) and the Bubble Tea UI. Blocks until quit.
func Run(ctx context.Context, cfg Config) error {
	store := memory.New(memory.Config{})
	isWSL := wsl.IsWSL2()

	var (
		engine capture.Engine
		mode   domain.CaptureMode
		attrs  *capture.Attributor
		ifaces []string
		privOK bool
		msg    string
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
			Mode:        mode,
			Running:     true,
			PrivilegeOK: privOK,
			Message:     msg,
			IsWSL2:      isWSL,
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

		// Prefer AF_PACKET; fall back to unprivileged /proc stats unless --strict-capture.
		if err := capture.CanOpenCapture(ifaces[0]); err != nil {
			if cfg.StrictCapture || !errors.Is(err, domain.ErrInsufficientPrivilege) {
				return err
			}
			mode = domain.ModeStats
			engine = &capture.ProcStatsEngine{Interval: time.Second}
			attrs = capture.NewAttributor(2 * time.Second)
			privOK = false
			msg = "stats mode (no CAP_NET_RAW): /proc counters, best-effort per-app"
			slog.Warn("falling back to unprivileged stats mode", "err", err)
		} else {
			mode = domain.ModeLive
			engine = &capture.LinuxEngine{}
			attrs = capture.NewAttributor(2 * time.Second)
			privOK = true
			msg = "packet capture (AF_PACKET)"
		}
		store.SetStatus(domain.Status{
			Mode:        mode,
			Running:     true,
			PrivilegeOK: privOK,
			Message:     msg,
			IsWSL2:      isWSL,
		})
	}

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	obs, errCh, err := engine.Start(runCtx, ifaces)
	if err != nil {
		return err
	}

	if attrs != nil {
		go attrs.Run(runCtx.Done())
	}

	// WSL2: sample Windows host adapter counters into store (inventory rates + windows-host app).
	if isWSL && !cfg.Demo {
		go sampleHostLoop(runCtx, store, isWSL, mode, privOK, msg)
	}

	// Ingest loop.
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

func sampleHostLoop(ctx context.Context, store *memory.Store, isWSL bool, mode domain.CaptureMode, privOK bool, baseMsg string) {
	prev := map[string]wsl.HostAdapterStats{}
	// First sample only seeds prev (no deltas).
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
			merged, nextPrev := wsl.MergeHostStatsIntoAdapters(adapters, stats, prev, 2.0)
			store.SetAdapters(merged)

			obs, next := wsl.HostTrafficObservations(now, stats, prev)
			prev = next
			_ = nextPrev
			for _, o := range obs {
				store.Ingest(o)
			}
			msg := baseMsg
			if msg != "" {
				msg += " · "
			}
			msg += "windows host counters"
			store.SetStatus(domain.Status{
				Mode:        mode,
				Running:     true,
				PrivilegeOK: privOK,
				Message:     msg,
				IsWSL2:      isWSL,
			})
		}
	}
}
