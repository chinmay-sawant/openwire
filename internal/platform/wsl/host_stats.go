package wsl

import (
	"context"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/chinmay-sawant/openwire/internal/domain"
)

// HostAdapterStats is a point-in-time Windows host adapter counter sample.
type HostAdapterStats struct {
	Name    string
	RxBytes uint64
	TxBytes uint64
	Up      bool
}

// SampleHostAdapterStats best-effort reads Windows adapter byte counters from WSL.
// Returns nil when not on WSL2 or when host tooling is unavailable.
func SampleHostAdapterStats(ctx context.Context) []HostAdapterStats {
	if !IsWSL2() {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	// ReceivedBytes / SentBytes are cumulative on modern Windows.
	script := `
$ErrorActionPreference='SilentlyContinue'
Get-NetAdapter | ForEach-Object {
  $s = Get-NetAdapterStatistics -Name $_.Name
  if ($null -ne $s) {
    '{0}|{1}|{2}|{3}' -f $_.Name, $_.Status, $s.ReceivedBytes, $s.SentBytes
  }
}
`
	cmd := exec.CommandContext(ctx, "powershell.exe", "-NoProfile", "-Command", script)
	out, err := cmd.Output()
	if err != nil || len(out) == 0 {
		return nil
	}
	var stats []HostAdapterStats
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Split(line, "|")
		if len(parts) < 4 {
			continue
		}
		rx, _ := strconv.ParseUint(strings.TrimSpace(parts[2]), 10, 64)
		tx, _ := strconv.ParseUint(strings.TrimSpace(parts[3]), 10, 64)
		stats = append(stats, HostAdapterStats{
			Name:    strings.TrimSpace(parts[0]),
			Up:      strings.EqualFold(strings.TrimSpace(parts[1]), "Up"),
			RxBytes: rx,
			TxBytes: tx,
		})
	}
	return stats
}

// MergeHostStatsIntoAdapters updates windows-host adapters with counters/rates.
// prev maps adapter name → previous sample; returns next map for the caller to keep.
func MergeHostStatsIntoAdapters(
	adapters []domain.Adapter,
	stats []HostAdapterStats,
	prev map[string]HostAdapterStats,
	elapsedSec float64,
) ([]domain.Adapter, map[string]HostAdapterStats) {
	if elapsedSec <= 0 {
		elapsedSec = 1
	}
	byName := make(map[string]HostAdapterStats, len(stats))
	for _, s := range stats {
		byName[s.Name] = s
	}
	nextPrev := make(map[string]HostAdapterStats, len(stats))
	out := make([]domain.Adapter, len(adapters))
	copy(out, adapters)
	for i := range out {
		if out[i].Source != domain.AdapterSourceWindowsHost {
			continue
		}
		s, ok := byName[out[i].Name]
		if !ok {
			continue
		}
		out[i].RxBytes = s.RxBytes
		out[i].TxBytes = s.TxBytes
		out[i].Up = s.Up
		if p, ok := prev[s.Name]; ok && elapsedSec > 0 {
			if s.RxBytes >= p.RxBytes {
				out[i].RxRateBps = float64(s.RxBytes-p.RxBytes) / elapsedSec
			}
			if s.TxBytes >= p.TxBytes {
				out[i].TxRateBps = float64(s.TxBytes-p.TxBytes) / elapsedSec
			}
		}
		nextPrev[s.Name] = s
	}
	// Keep prev for adapters missing this round so rates can resume.
	for k, v := range prev {
		if _, ok := nextPrev[k]; !ok {
			nextPrev[k] = v
		}
	}
	return out, nextPrev
}

// HostTrafficObservations converts host adapter deltas into store-friendly observations
// attributed to a synthetic "windows-host" application (not a Linux PID).
// Prefer HostTrafficObservationsByProcess when a Windows process list is available.
func HostTrafficObservations(
	now time.Time,
	stats []HostAdapterStats,
	prev map[string]HostAdapterStats,
) (obs []domain.Observation, next map[string]HostAdapterStats) {
	return HostTrafficObservationsByProcess(now, stats, prev, nil)
}

func clampInt(n uint64) int {
	const max = 1 << 30
	if n > max {
		return max
	}
	return int(n)
}
