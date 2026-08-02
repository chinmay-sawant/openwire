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
		msg = "demo mode"
		store.SetStatus(domain.Status{
			Mode:        mode,
			Running:     true,
			PrivilegeOK: true,
			Message:     msg,
			IsWSL2:      isWSL,
		})
	} else {
		mode = domain.ModeLive
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
		if err := capture.CanOpenCapture(ifaces[0]); err != nil {
			return err
		}
		engine = &capture.LinuxEngine{}
		attrs = capture.NewAttributor(2 * time.Second)
		store.SetStatus(domain.Status{
			Mode:        mode,
			Running:     true,
			PrivilegeOK: true,
			Message:     "capturing",
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
						PrivilegeOK: !errors.Is(err, domain.ErrInsufficientPrivilege),
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
