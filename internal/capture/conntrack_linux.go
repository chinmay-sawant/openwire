//go:build linux

package capture

import (
	"bufio"
	"os"
	"strconv"
	"strings"

	"github.com/chinmay-sawant/openwire/internal/domain"
)

// connEntry is one nf_conntrack / ip_conntrack row (original direction).
type connEntry struct {
	proto domain.Protocol
	src   string
	dst   string
	sport uint16
	dport uint16
	bytes uint64
	pkts  uint64
}

// readConntrack parses /proc/net/nf_conntrack or /proc/net/ip_conntrack.
func readConntrack() ([]connEntry, error) {
	paths := []string{"/proc/net/nf_conntrack", "/proc/net/ip_conntrack"}
	var lastErr error
	for _, p := range paths {
		f, err := os.Open(p)
		if err != nil {
			lastErr = err
			continue
		}
		entries, err := parseConntrack(f)
		_ = f.Close()
		if err != nil {
			lastErr = err
			continue
		}
		return entries, nil
	}
	if lastErr == nil {
		lastErr = os.ErrNotExist
	}
	return nil, lastErr
}

func parseConntrack(f *os.File) ([]connEntry, error) {
	var out []connEntry
	sc := bufio.NewScanner(f)
	// Large lines possible.
	buf := make([]byte, 0, 64*1024)
	sc.Buffer(buf, 1024*1024)
	for sc.Scan() {
		line := sc.Text()
		e, ok := parseConntrackLine(line)
		if ok {
			out = append(out, e)
		}
	}
	return out, sc.Err()
}

// parseConntrackLine extracts the original tuple and its bytes counter.
// Example fragment:
//
//	ipv4 2 tcp 6 431999 ESTABLISHED src=10.0.0.2 dst=1.1.1.1 sport=45678 dport=443 packets=10 bytes=1234 src=... dst=... sport=443 dport=45678 packets=8 bytes=900 [ASSURED]
func parseConntrackLine(line string) (connEntry, bool) {
	var e connEntry
	fields := strings.Fields(line)
	if len(fields) < 10 {
		return e, false
	}
	// find protocol token
	proto := domain.ProtoOther
	for _, f := range fields {
		switch f {
		case "tcp":
			proto = domain.ProtoTCP
		case "udp":
			proto = domain.ProtoUDP
		case "icmp":
			proto = domain.ProtoICMP
		}
		if proto != domain.ProtoOther {
			break
		}
	}
	if proto == domain.ProtoOther {
		return e, false
	}
	e.proto = proto

	// First src=/dst=/sport=/dport=/bytes= group is original direction.
	gotSrc, gotDst, gotSp, gotDp, gotBytes := false, false, false, false, false
	for _, f := range fields {
		if strings.HasPrefix(f, "src=") && !gotSrc {
			e.src = strings.TrimPrefix(f, "src=")
			gotSrc = true
			continue
		}
		if strings.HasPrefix(f, "dst=") && !gotDst {
			e.dst = strings.TrimPrefix(f, "dst=")
			gotDst = true
			continue
		}
		if strings.HasPrefix(f, "sport=") && !gotSp {
			if v, err := strconv.ParseUint(strings.TrimPrefix(f, "sport="), 10, 16); err == nil {
				e.sport = uint16(v)
				gotSp = true
			}
			continue
		}
		if strings.HasPrefix(f, "dport=") && !gotDp {
			if v, err := strconv.ParseUint(strings.TrimPrefix(f, "dport="), 10, 16); err == nil {
				e.dport = uint16(v)
				gotDp = true
			}
			continue
		}
		if strings.HasPrefix(f, "bytes=") && !gotBytes {
			if v, err := strconv.ParseUint(strings.TrimPrefix(f, "bytes="), 10, 64); err == nil {
				e.bytes = v
				gotBytes = true
			}
			// after first bytes we have original direction complete
			if gotSrc && gotDst {
				break
			}
		}
		if strings.HasPrefix(f, "packets=") && e.pkts == 0 {
			if v, err := strconv.ParseUint(strings.TrimPrefix(f, "packets="), 10, 64); err == nil {
				e.pkts = v
			}
		}
	}
	if !gotSrc || !gotDst {
		return e, false
	}
	return e, true
}

// ConntrackAvailable reports whether conntrack accounting is readable.
func ConntrackAvailable() bool {
	_, err := readConntrack()
	return err == nil
}
