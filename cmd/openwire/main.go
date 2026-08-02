// Command openwire is the OpenWire network traffic monitor entrypoint.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/chinmay-sawant/openwire/internal/app"
	"github.com/chinmay-sawant/openwire/internal/platform/logging"
	"github.com/spf13/cobra"
)

func main() {
	root := &cobra.Command{
		Use:           "openwire",
		Short:         "OpenWire — terminal network traffic monitor",
		Long:          "Detect local network traffic, keep it in memory, and show per-app bandwidth in a Bubble Tea UI.",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	var (
		demo          bool
		ifaces        []string
		theme         string
		logLevel      string
		strictCapture bool
		dbPath        string
		noDB          bool
		pcapPath      string
		pcapMaxMB     int
		deep          bool
		ebpf          bool
	)

	start := &cobra.Command{
		Use:   "start",
		Short: "Start the OpenWire monitor UI",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, closer, _ := logging.Setup(logLevel)
			if closer != nil {
				defer closer.Close()
			}

			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer stop()

			return app.Run(ctx, app.Config{
				Demo:          demo,
				Ifaces:        ifaces,
				Theme:         theme,
				LogLevel:      logLevel,
				StrictCapture: strictCapture,
				DBPath:        dbPath,
				NoDB:          noDB,
				PCAPPath:      pcapPath,
				PCAPMaxMB:     pcapMaxMB,
				Deep:          deep,
				EBPF:          ebpf,
			})
		},
	}

	start.Flags().BoolVar(&demo, "demo", false, "use synthetic traffic (no capture privileges required)")
	start.Flags().StringSliceVar(&ifaces, "iface", nil, "capture only these interfaces (repeatable)")
	start.Flags().StringVar(&theme, "theme", "dark", "UI theme (default: dark)")
	start.Flags().StringVar(&logLevel, "log-level", "info", "log level: debug|info|warn|error")
	start.Flags().BoolVar(&strictCapture, "strict-capture", false, "require AF_PACKET privileges; do not fall back to /proc stats mode")
	start.Flags().StringVar(&dbPath, "db", "", "SQLite history path (default: $XDG_STATE_HOME/openwire/openwire.db)")
	start.Flags().BoolVar(&noDB, "no-db", false, "disable SQLite persistence")
	start.Flags().StringVar(&pcapPath, "pcap", "", "write Wireshark-compatible packet capture to this path (live AF_PACKET only)")
	start.Flags().IntVar(&pcapMaxMB, "pcap-max-mb", 256, "rotate pcap after this many MiB (keeps .1 backup)")
	start.Flags().BoolVar(&deep, "deep", false, "prefer conntrack deep attribution when AF_PACKET is unavailable")
	start.Flags().BoolVar(&ebpf, "ebpf", false, "enable best-effort eBPF kprobe counters (needs CAP_BPF/root + tracefs)")

	root.AddCommand(start)

	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "openwire:", err)
		os.Exit(1)
	}
}
