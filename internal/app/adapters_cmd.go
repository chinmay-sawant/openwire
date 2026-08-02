package app

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/chinmay-sawant/openwire/internal/capture"
	"github.com/chinmay-sawant/openwire/internal/domain"
	"github.com/chinmay-sawant/openwire/internal/platform/wsl"
)

// AdaptersOptions controls the adapters command output.
type AdaptersOptions struct {
	// JSON reserved for later; currently table only.
	Out io.Writer
}

// PrintAdapters lists Linux and (on WSL2) Windows host adapters as a table.
func PrintAdapters(ctx context.Context, opt AdaptersOptions) error {
	out := opt.Out
	if out == nil {
		out = os.Stdout
	}

	isWSL := wsl.IsWSL2()
	fmt.Fprintf(out, "OpenWire adapters\n")
	fmt.Fprintf(out, "  environment: %s\n", envLabel(isWSL))
	fmt.Fprintln(out)

	linux, err := capture.ListLinuxAdapters()
	if err != nil {
		return fmt.Errorf("list linux adapters: %w", err)
	}

	// Enrich Linux with /proc/net/dev counters when available (same host).
	if counters, cErr := readLinuxDevCounters(); cErr == nil {
		for i := range linux {
			if c, ok := counters[linux[i].Name]; ok {
				linux[i].RxBytes = c.rx
				linux[i].TxBytes = c.tx
			}
		}
	}

	var windows []domain.Adapter
	if isWSL {
		windows = wsl.ListWindowsHostAdapters(ctx)
		if stats := wsl.SampleHostAdapterStats(ctx); len(stats) > 0 {
			byName := make(map[string]wsl.HostAdapterStats, len(stats))
			for _, s := range stats {
				byName[s.Name] = s
			}
			for i := range windows {
				if s, ok := byName[windows[i].Name]; ok {
					windows[i].RxBytes = s.RxBytes
					windows[i].TxBytes = s.TxBytes
					windows[i].Up = s.Up
				}
			}
		}
	}

	tw := tabwriter.NewWriter(out, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "SOURCE\tNAME\tSTATE\tIPv4\tMAC\tRX\tTX\tNOTES")
	fmt.Fprintln(tw, "------\t----\t-----\t----\t---\t--\t--\t-----")

	for _, a := range linux {
		printAdapterRow(tw, a, linuxNote(a, isWSL))
	}
	for _, a := range windows {
		printAdapterRow(tw, a, windowsNote(a))
	}
	if err := tw.Flush(); err != nil {
		return err
	}

	fmt.Fprintln(out)
	fmt.Fprintf(out, "totals: %d linux", len(linux))
	if isWSL {
		fmt.Fprintf(out, ", %d windows-host", len(windows))
		if len(windows) == 0 {
			fmt.Fprint(out, " (host tools unavailable — is /mnt/c mounted?)")
		}
	}
	fmt.Fprintln(out)

	if isWSL {
		fmt.Fprintln(out, "tip: Windows internet is usually Wi-Fi/Ethernet; WSL uses eth0 bridged via vEthernet (WSL).")
		fmt.Fprintln(out, "     Per-process Windows usage appears in the TUI as win/<name> when host stats work.")
	}
	return nil
}

func envLabel(isWSL bool) string {
	if isWSL {
		return "WSL2 (Linux guest + Windows host)"
	}
	return "Linux"
}

func printAdapterRow(tw *tabwriter.Writer, a domain.Adapter, note string) {
	state := "down"
	if a.Up {
		state = "up"
	}
	if a.Loopback {
		state += "/lo"
	}
	src := string(a.Source)
	if src == "" {
		src = "linux"
	}
	ipv4 := strings.Join(a.IPv4, ",")
	if ipv4 == "" {
		ipv4 = "-"
	}
	mac := a.Hardware
	if mac == "" {
		mac = "-"
	}
	fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
		src,
		a.Name,
		state,
		ipv4,
		mac,
		humanBytes(a.RxBytes),
		humanBytes(a.TxBytes),
		note,
	)
}

func linuxNote(a domain.Adapter, isWSL bool) string {
	switch {
	case a.Loopback:
		return "loopback"
	case isWSL && a.Name == "eth0" && a.Up:
		return "WSL virtual NIC (→ host vEthernet)"
	case strings.HasPrefix(a.Name, "docker"):
		return "docker bridge"
	case a.Up:
		return "capture target"
	default:
		return ""
	}
}

func windowsNote(a domain.Adapter) string {
	n := strings.ToLower(a.Name)
	switch {
	case strings.Contains(n, "wsl") || strings.Contains(n, "hyper-v"):
		return "WSL host bridge"
	case strings.Contains(n, "wi-fi") || strings.Contains(n, "wifi"):
		if a.Up {
			return "likely primary internet"
		}
		return "wifi"
	case n == "ethernet" || strings.HasPrefix(n, "ethernet "):
		if a.Up {
			return "wired"
		}
		return "wired (disconnected)"
	case strings.Contains(n, "vmware") || strings.Contains(n, "virtualbox"):
		return "hypervisor"
	case strings.Contains(n, "bluetooth"):
		return "bluetooth"
	default:
		return ""
	}
}

type devCounters struct{ rx, tx uint64 }

// readLinuxDevCounters is a tiny /proc/net/dev reader for the adapters CLI.
func readLinuxDevCounters() (map[string]devCounters, error) {
	// Avoid importing capture internals; duplicate minimal parse.
	data, err := os.ReadFile("/proc/net/dev")
	if err != nil {
		return nil, err
	}
	out := make(map[string]devCounters)
	lines := strings.Split(string(data), "\n")
	for i, line := range lines {
		if i < 2 {
			continue
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		col := strings.IndexByte(line, ':')
		if col < 0 {
			continue
		}
		name := strings.TrimSpace(line[:col])
		fields := strings.Fields(line[col+1:])
		if len(fields) < 9 {
			continue
		}
		var rx, tx uint64
		fmt.Sscanf(fields[0], "%d", &rx)
		fmt.Sscanf(fields[8], "%d", &tx)
		out[name] = devCounters{rx: rx, tx: tx}
	}
	return out, nil
}

func humanBytes(n uint64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := uint64(unit), 0
	for v := n / unit; v >= unit; v /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(n)/float64(div), "KMGTPE"[exp])
}

// Ensure context has a sensible timeout for host PowerShell calls.
func AdaptersContext(parent context.Context) (context.Context, context.CancelFunc) {
	if parent == nil {
		parent = context.Background()
	}
	return context.WithTimeout(parent, 15*time.Second)
}
