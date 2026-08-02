//go:build linux

package capture

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/chinmay-sawant/openwire/internal/domain"
)

// Attributor maps local sockets to process name/pid using /proc.
type Attributor struct {
	mu      sync.RWMutex
	byTuple map[string]attrInfo
	every   time.Duration
}

type attrInfo struct {
	pid  int
	name string
	path string
}

// NewAttributor creates a process attributor.
func NewAttributor(every time.Duration) *Attributor {
	if every <= 0 {
		every = 2 * time.Second
	}
	return &Attributor{
		byTuple: make(map[string]attrInfo),
		every:   every,
	}
}

// Run periodically refreshes the socket→process map until ctx is done.
func (a *Attributor) Run(done <-chan struct{}) {
	a.refresh()
	t := time.NewTicker(a.every)
	defer t.Stop()
	for {
		select {
		case <-done:
			return
		case <-t.C:
			a.refresh()
		}
	}
}

// Annotate fills AppHint/PIDHint when known.
// Prefer real process/binary names over empty or "unknown".
func (a *Attributor) Annotate(o *domain.Observation) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	keys := []string{
		tupleKey(o.Protocol, o.SrcIP, o.SrcPort),
		tupleKey(o.Protocol, o.DstIP, o.DstPort),
	}
	for _, k := range keys {
		if info, ok := a.byTuple[k]; ok {
			if o.PIDHint == 0 {
				o.PIDHint = info.pid
			}
			if needsAppName(o.AppHint) {
				o.AppHint = preferredName(info.name, info.path)
			}
			return
		}
	}
	// Last resort: PID → binary if we already have a pid hint.
	if o.PIDHint > 0 && needsAppName(o.AppHint) {
		name, path := procName(o.PIDHint)
		o.AppHint = preferredName(name, path)
	}
}

func needsAppName(hint string) bool {
	return hint == "" || hint == "unknown" || strings.HasPrefix(hint, "pid:")
}

func preferredName(name, path string) string {
	if path != "" {
		base := filepath.Base(path)
		// Prefer executable basename when comm is generic/empty.
		if base != "" && base != "." {
			if name == "" || name == "unknown" || strings.HasPrefix(name, "pid:") {
				return base
			}
			// Prefer longer/more specific binary name when different.
			if len(base) >= len(name) {
				return base
			}
		}
	}
	if name != "" {
		return name
	}
	if path != "" {
		return filepath.Base(path)
	}
	return "unknown"
}

func (a *Attributor) refresh() {
	inodeToPID := mapInodeToPID()
	next := make(map[string]attrInfo, 1024)
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
			// skip header
		}
		for sc.Scan() {
			fields := strings.Fields(sc.Text())
			if len(fields) < 10 {
				continue
			}
			ip, port, ok := parseProcAddr(fields[1])
			if !ok {
				continue
			}
			pid, ok := inodeToPID[fields[9]]
			if !ok {
				continue
			}
			name, path := procName(pid)
			next[tupleKey(proto, ip, port)] = attrInfo{pid: pid, name: name, path: path}
		}
		_ = f.Close()
	}
	a.mu.Lock()
	a.byTuple = next
	a.mu.Unlock()
}

func tupleKey(proto domain.Protocol, ip string, port uint16) string {
	return string(proto) + "|" + ip + "|" + strconv.Itoa(int(port))
}

func parseProcAddr(s string) (ip string, port uint16, ok bool) {
	parts := strings.Split(s, ":")
	if len(parts) != 2 {
		return "", 0, false
	}
	port64, err := strconv.ParseUint(parts[1], 16, 16)
	if err != nil {
		return "", 0, false
	}
	port = uint16(port64)
	hip := parts[0]
	switch len(hip) {
	case 8:
		v, err := strconv.ParseUint(hip, 16, 32)
		if err != nil {
			return "", 0, false
		}
		ip = fmt.Sprintf("%d.%d.%d.%d", byte(v), byte(v>>8), byte(v>>16), byte(v>>24))
		return ip, port, true
	case 32:
		var b [16]byte
		for i := 0; i < 4; i++ {
			chunk := hip[i*8 : i*8+8]
			v, err := strconv.ParseUint(chunk, 16, 32)
			if err != nil {
				return "", 0, false
			}
			b[i*4+0] = byte(v)
			b[i*4+1] = byte(v >> 8)
			b[i*4+2] = byte(v >> 16)
			b[i*4+3] = byte(v >> 24)
		}
		ip = net.IP(b[:]).String()
		return ip, port, true
	default:
		return "", 0, false
	}
}

func mapInodeToPID() map[string]int {
	out := make(map[string]int, 256)
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return out
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		pid, err := strconv.Atoi(e.Name())
		if err != nil {
			continue
		}
		fdDir := filepath.Join("/proc", e.Name(), "fd")
		fds, err := os.ReadDir(fdDir)
		if err != nil {
			continue
		}
		for _, fd := range fds {
			link, err := os.Readlink(filepath.Join(fdDir, fd.Name()))
			if err != nil {
				continue
			}
			if strings.HasPrefix(link, "socket:[") && strings.HasSuffix(link, "]") {
				ino := link[len("socket:[") : len(link)-1]
				if _, exists := out[ino]; !exists {
					out[ino] = pid
				}
			}
		}
	}
	return out
}

// ProcessName returns the process comm/exe basename for a PID (Linux).
func ProcessName(pid int) string {
	name, _ := procName(pid)
	return name
}

func procName(pid int) (name, path string) {
	base := filepath.Join("/proc", strconv.Itoa(pid))
	if b, err := os.ReadFile(filepath.Join(base, "comm")); err == nil {
		name = strings.TrimSpace(string(b))
	}
	if p, err := os.Readlink(filepath.Join(base, "exe")); err == nil {
		path = p
		// Prefer binary basename as the display name when available.
		if bn := filepath.Base(p); bn != "" && bn != "." {
			name = bn
		}
	}
	if name == "" {
		name = "pid:" + strconv.Itoa(pid)
	}
	return name, path
}
