package wsl

import (
	"context"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/chinmay-sawant/openwire/internal/domain"
)

// WinProcess is a Windows host process that currently owns network endpoints.
type WinProcess struct {
	PID       int
	Name      string
	ConnCount int
	// Optional sample endpoints for flow display.
	LocalIP   string
	LocalPort uint16
	RemoteIP  string
	RemotePort uint16
	Protocol  domain.Protocol
}

// ListWindowsProcesses best-effort inventories Windows processes with open TCP/UDP endpoints.
// Returns nil when not on WSL2 or when host tooling is unavailable.
func ListWindowsProcesses(ctx context.Context) []WinProcess {
	if !IsWSL2() {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()

	// One PowerShell invocation: map PID→name, then emit connection rows.
	script := `
$ErrorActionPreference='SilentlyContinue'
$names = @{}
Get-Process | ForEach-Object { $names[$_.Id] = $_.ProcessName }
function Emit-Row($proto, $pid, $la, $lp, $ra, $rp) {
  if ($null -eq $pid -or $pid -le 0) { return }
  $n = $names[$pid]
  if (-not $n) { $n = "pid:$pid" }
  if (-not $la) { $la = '' }
  if (-not $ra) { $ra = '' }
  if ($null -eq $lp) { $lp = 0 }
  if ($null -eq $rp) { $rp = 0 }
  '{0}|{1}|{2}|{3}|{4}|{5}|{6}' -f $pid, $n, $la, $lp, $ra, $rp, $proto
}
Get-NetTCPConnection | ForEach-Object {
  Emit-Row 'tcp' $_.OwningProcess $_.LocalAddress $_.LocalPort $_.RemoteAddress $_.RemotePort
}
Get-NetUDPEndpoint | ForEach-Object {
  Emit-Row 'udp' $_.OwningProcess $_.LocalAddress $_.LocalPort '' 0
}
`
	cmd := exec.CommandContext(ctx, "powershell.exe", "-NoProfile", "-Command", script)
	out, err := cmd.Output()
	if err != nil || len(out) == 0 {
		return nil
	}
	return parseWindowsProcessRows(string(out))
}

// parseWindowsProcessRows aggregates pipe-delimited rows into weighted processes.
// Exported for tests via package-level use (same package tests).
func parseWindowsProcessRows(raw string) []WinProcess {
	type acc struct {
		p     WinProcess
		count int
	}
	byPID := map[int]*acc{}
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Split(line, "|")
		if len(parts) < 7 {
			continue
		}
		pid, err := strconv.Atoi(strings.TrimSpace(parts[0]))
		if err != nil || pid <= 0 {
			continue
		}
		name := strings.TrimSpace(parts[1])
		if name == "" {
			name = "pid:" + strconv.Itoa(pid)
		}
		a, ok := byPID[pid]
		if !ok {
			proto := domain.ProtoTCP
			if strings.EqualFold(strings.TrimSpace(parts[6]), "udp") {
				proto = domain.ProtoUDP
			}
			lp, _ := strconv.Atoi(strings.TrimSpace(parts[3]))
			rp, _ := strconv.Atoi(strings.TrimSpace(parts[5]))
			a = &acc{p: WinProcess{
				PID:        pid,
				Name:       name,
				LocalIP:    strings.TrimSpace(parts[2]),
				LocalPort:  uint16(lp),
				RemoteIP:   strings.TrimSpace(parts[4]),
				RemotePort: uint16(rp),
				Protocol:   proto,
			}}
			byPID[pid] = a
		}
		a.count++
		a.p.ConnCount = a.count
	}
	out := make([]WinProcess, 0, len(byPID))
	for _, a := range byPID {
		out = append(out, a.p)
	}
	return out
}

// HostTrafficObservationsByProcess distributes host adapter byte deltas across
// Windows processes by connection count. Falls back to aggregate windows-host
// when processes is empty.
func HostTrafficObservationsByProcess(
	now time.Time,
	stats []HostAdapterStats,
	prev map[string]HostAdapterStats,
	processes []WinProcess,
) (obs []domain.Observation, next map[string]HostAdapterStats) {
	// Always advance prev map.
	next = make(map[string]HostAdapterStats, len(stats))
	var totalRx, totalTx uint64
	var sampleIface string
	for _, s := range stats {
		next[s.Name] = s
		p, ok := prev[s.Name]
		if !ok {
			continue
		}
		var drx, dtx uint64
		if s.RxBytes >= p.RxBytes {
			drx = s.RxBytes - p.RxBytes
		}
		if s.TxBytes >= p.TxBytes {
			dtx = s.TxBytes - p.TxBytes
		}
		totalRx += drx
		totalTx += dtx
		if sampleIface == "" {
			sampleIface = "win:" + s.Name
		}
	}
	for k, v := range prev {
		if _, ok := next[k]; !ok {
			next[k] = v
		}
	}

	if totalRx == 0 && totalTx == 0 {
		return nil, next
	}
	if sampleIface == "" {
		sampleIface = "win:host"
	}

	if len(processes) == 0 {
		return hostAggregateObs(now, sampleIface, totalRx, totalTx), next
	}

	totalConns := 0
	for _, p := range processes {
		if p.ConnCount > 0 {
			totalConns += p.ConnCount
		} else {
			totalConns++
		}
	}
	if totalConns <= 0 {
		return hostAggregateObs(now, sampleIface, totalRx, totalTx), next
	}

	var assignedRx, assignedTx uint64
	for i, p := range processes {
		weight := p.ConnCount
		if weight <= 0 {
			weight = 1
		}
		var rxShare, txShare uint64
		if i < len(processes)-1 {
			rxShare = totalRx * uint64(weight) / uint64(totalConns)
			txShare = totalTx * uint64(weight) / uint64(totalConns)
			assignedRx += rxShare
			assignedTx += txShare
		} else {
			rxShare = totalRx - assignedRx
			txShare = totalTx - assignedTx
		}
		app := "win/" + sanitizeAppName(p.Name)
		if rxShare > 0 {
			obs = append(obs, domain.Observation{
				Time:      now,
				Iface:     sampleIface,
				Direction: domain.DirectionRx,
				Length:    clampInt(rxShare),
				Protocol:  p.Protocol,
				SrcIP:     p.RemoteIP,
				SrcPort:   p.RemotePort,
				DstIP:     p.LocalIP,
				DstPort:   p.LocalPort,
				AppHint:   app,
				PIDHint:   p.PID,
			})
		}
		if txShare > 0 {
			obs = append(obs, domain.Observation{
				Time:      now,
				Iface:     sampleIface,
				Direction: domain.DirectionTx,
				Length:    clampInt(txShare),
				Protocol:  p.Protocol,
				SrcIP:     p.LocalIP,
				SrcPort:   p.LocalPort,
				DstIP:     p.RemoteIP,
				DstPort:   p.RemotePort,
				AppHint:   app,
				PIDHint:   p.PID,
			})
		}
	}
	return obs, next
}

func hostAggregateObs(now time.Time, iface string, rx, tx uint64) []domain.Observation {
	var obs []domain.Observation
	if rx > 0 {
		obs = append(obs, domain.Observation{
			Time: now, Iface: iface, Direction: domain.DirectionRx,
			Length: clampInt(rx), Protocol: domain.ProtoOther, AppHint: "windows-host",
		})
	}
	if tx > 0 {
		obs = append(obs, domain.Observation{
			Time: now, Iface: iface, Direction: domain.DirectionTx,
			Length: clampInt(tx), Protocol: domain.ProtoOther, AppHint: "windows-host",
		})
	}
	return obs
}

func sanitizeAppName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "unknown"
	}
	// Avoid pipe/control noise in UI keys.
	name = strings.ReplaceAll(name, "|", "_")
	name = strings.ReplaceAll(name, "#", "_")
	return name
}
