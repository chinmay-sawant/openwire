package wsl

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/chinmay-sawant/openwire/internal/domain"
)

// WinProcess is a Windows host process that currently owns network endpoints.
type WinProcess struct {
	PID        int
	Name       string
	ConnCount  int
	LocalIP    string
	LocalPort  uint16
	RemoteIP   string
	RemotePort uint16
	Protocol   domain.Protocol
}

var (
	procCacheMu sync.Mutex
	procCache   []WinProcess
	procCacheAt time.Time
)

// ListWindowsProcesses best-effort inventories Windows processes with open TCP/UDP endpoints.
// Results are cached briefly to keep the UI loop responsive.
func ListWindowsProcesses(ctx context.Context) []WinProcess {
	if !IsWSL2() {
		return nil
	}
	procCacheMu.Lock()
	if time.Since(procCacheAt) < 5*time.Second && len(procCache) > 0 {
		out := append([]WinProcess(nil), procCache...)
		procCacheMu.Unlock()
		return out
	}
	procCacheMu.Unlock()

	ps := powershellPath()
	if ps == "" {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	// Resolve names with Get-Process (reliable). Hashtable key = [int]Id.
	// Avoid Get-CimInstance map lookups which often miss OwningProcess keys from WSL.
	script := `
$ErrorActionPreference = 'SilentlyContinue'
$names = @{}
Get-Process | ForEach-Object { $names[[int]$_.Id] = [string]$_.ProcessName }
function Emit-Row([string]$proto, $opid, $la, $lp, $ra, $rp) {
  try { $pid = [int]$opid } catch { return }
  if ($pid -le 0) { return }
  $n = $names[$pid]
  if ([string]::IsNullOrWhiteSpace($n)) {
    try {
      $n = [string](Get-Process -Id $pid -ErrorAction Stop).ProcessName
      $names[$pid] = $n
    } catch {
      $n = ''
    }
  }
  if ([string]::IsNullOrWhiteSpace($n)) { $n = "pid:$pid" }
  if (-not $la) { $la = '' }
  if (-not $ra) { $ra = '' }
  if ($null -eq $lp) { $lp = 0 }
  if ($null -eq $rp) { $rp = 0 }
  '{0}|{1}|{2}|{3}|{4}|{5}|{6}' -f $pid, $n, $la, $lp, $ra, $rp, $proto
}
Get-NetTCPConnection -State Established -ErrorAction SilentlyContinue | ForEach-Object {
  Emit-Row 'tcp' $_.OwningProcess $_.LocalAddress $_.LocalPort $_.RemoteAddress $_.RemotePort
}
Get-NetUDPEndpoint -ErrorAction SilentlyContinue | ForEach-Object {
  Emit-Row 'udp' $_.OwningProcess $_.LocalAddress $_.LocalPort '' 0
}
`
	cmd := exec.CommandContext(ctx, ps, "-NoProfile", "-Command", script)
	out, err := cmd.Output()
	if err != nil || len(out) == 0 {
		out2, err2 := listProcessesFallback(ctx, ps)
		if err2 != nil || len(out2) == 0 {
			return nil
		}
		out = out2
	}
	procs := parseWindowsProcessRows(string(out))
	// Second pass: resolve any remaining pid:N via individual Get-Process.
	procs = resolveMissingNames(ctx, ps, procs)

	procCacheMu.Lock()
	procCache = procs
	procCacheAt = time.Now()
	procCacheMu.Unlock()
	return procs
}

func listProcessesFallback(ctx context.Context, ps string) ([]byte, error) {
	script := `
$ErrorActionPreference='SilentlyContinue'
$names=@{}
Get-Process | ForEach-Object { $names[[int]$_.Id]=[string]$_.ProcessName }
Get-NetTCPConnection -ErrorAction SilentlyContinue | ForEach-Object {
  $pid=[int]$_.OwningProcess
  if ($pid -le 0) { return }
  $n=$names[$pid]
  if ([string]::IsNullOrWhiteSpace($n)) { $n = "pid:$pid" }
  "{0}|{1}|{2}|{3}|{4}|{5}|tcp" -f $pid,$n,$_.LocalAddress,$_.LocalPort,$_.RemoteAddress,$_.RemotePort
}
`
	cmd := exec.CommandContext(ctx, ps, "-NoProfile", "-Command", script)
	return cmd.Output()
}

// resolveMissingNames fixes rows that only have pid:N by querying Get-Process.
func resolveMissingNames(ctx context.Context, ps string, procs []WinProcess) []WinProcess {
	var missing []int
	for _, p := range procs {
		if strings.HasPrefix(p.Name, "pid:") || p.Name == "" {
			missing = append(missing, p.PID)
		}
	}
	if len(missing) == 0 {
		return procs
	}
	// Cap bulk resolve.
	if len(missing) > 40 {
		missing = missing[:40]
	}
	ids := make([]string, len(missing))
	for i, id := range missing {
		ids[i] = strconv.Itoa(id)
	}
	script := fmt.Sprintf(`
$ErrorActionPreference='SilentlyContinue'
@(%s) | ForEach-Object {
  $p = Get-Process -Id $_ -ErrorAction SilentlyContinue
  if ($p) { "{0}|{1}" -f $_, $p.ProcessName }
}
`, strings.Join(ids, ","))
	cmd := exec.CommandContext(ctx, ps, "-NoProfile", "-Command", script)
	out, err := cmd.Output()
	if err != nil || len(out) == 0 {
		return procs
	}
	resolved := map[int]string{}
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		parts := strings.SplitN(line, "|", 2)
		if len(parts) != 2 {
			continue
		}
		pid, err := strconv.Atoi(parts[0])
		if err != nil {
			continue
		}
		name := strings.TrimSpace(parts[1])
		if name != "" {
			resolved[pid] = name
		}
	}
	for i := range procs {
		if n, ok := resolved[procs[i].PID]; ok {
			procs[i].Name = n
		}
	}
	return procs
}

// parseWindowsProcessRows aggregates pipe-delimited rows into weighted processes.
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
		name = strings.TrimSuffix(name, ".exe")
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
		} else if strings.HasPrefix(a.p.Name, "pid:") && !strings.HasPrefix(name, "pid:") {
			a.p.Name = name
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

// DisplayName returns "name · pid" for UI lists.
func (p WinProcess) DisplayName() string {
	n := sanitizeAppName(p.Name)
	if p.PID > 0 {
		return fmt.Sprintf("%s · %d", n, p.PID)
	}
	return n
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
	next = make(map[string]HostAdapterStats, len(stats))
	type delta struct {
		name string
		rx   uint64
		tx   uint64
	}
	var deltas []delta
	var totalRx, totalTx uint64
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
		if drx == 0 && dtx == 0 {
			continue
		}
		totalRx += drx
		totalTx += dtx
		deltas = append(deltas, delta{name: s.Name, rx: drx, tx: dtx})
	}
	for k, v := range prev {
		if _, ok := next[k]; !ok {
			next[k] = v
		}
	}

	if totalRx == 0 && totalTx == 0 {
		return nil, next
	}

	if len(processes) == 0 {
		for _, d := range deltas {
			iface := "win:" + d.name
			obs = append(obs, hostAggregateObs(now, iface, d.rx, d.tx)...)
		}
		return obs, next
	}

	totalConns := 0
	for _, p := range processes {
		w := p.ConnCount
		if w <= 0 {
			w = 1
		}
		totalConns += w
	}
	if totalConns <= 0 {
		for _, d := range deltas {
			obs = append(obs, hostAggregateObs(now, "win:"+d.name, d.rx, d.tx)...)
		}
		return obs, next
	}

	for _, d := range deltas {
		iface := "win:" + d.name
		var assignedRx, assignedTx uint64
		for i, p := range processes {
			weight := p.ConnCount
			if weight <= 0 {
				weight = 1
			}
			var rxShare, txShare uint64
			if i < len(processes)-1 {
				rxShare = d.rx * uint64(weight) / uint64(totalConns)
				txShare = d.tx * uint64(weight) / uint64(totalConns)
				assignedRx += rxShare
				assignedTx += txShare
			} else {
				rxShare = d.rx - assignedRx
				txShare = d.tx - assignedTx
			}
			// AppHint: win/<name> — PID carried separately so UI can show "name · pid".
			app := "win/" + sanitizeAppName(p.Name)
			if rxShare > 0 {
				obs = append(obs, domain.Observation{
					Time: now, Iface: iface, Direction: domain.DirectionRx,
					Length: clampInt(rxShare), Protocol: p.Protocol,
					SrcIP: p.RemoteIP, SrcPort: p.RemotePort,
					DstIP: p.LocalIP, DstPort: p.LocalPort,
					AppHint: app, PIDHint: p.PID,
				})
			}
			if txShare > 0 {
				obs = append(obs, domain.Observation{
					Time: now, Iface: iface, Direction: domain.DirectionTx,
					Length: clampInt(txShare), Protocol: p.Protocol,
					SrcIP: p.LocalIP, SrcPort: p.LocalPort,
					DstIP: p.RemoteIP, DstPort: p.RemotePort,
					AppHint: app, PIDHint: p.PID,
				})
			}
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
	name = strings.TrimSuffix(name, ".exe")
	if name == "" {
		return "unknown"
	}
	// Strip accidental pid: prefix if name resolution later filled real name elsewhere.
	name = strings.ReplaceAll(name, "|", "_")
	name = strings.ReplaceAll(name, "#", "_")
	return name
}
