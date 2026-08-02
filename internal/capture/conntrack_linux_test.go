//go:build linux

package capture

import (
	"strings"
	"testing"

	"github.com/chinmay-sawant/openwire/internal/domain"
)

func TestParseConntrackLine(t *testing.T) {
	line := `ipv4     2 tcp      6 431999 ESTABLISHED src=10.0.0.2 dst=1.1.1.1 sport=45678 dport=443 packets=10 bytes=5000 src=1.1.1.1 dst=10.0.0.2 sport=443 dport=45678 packets=8 bytes=900 [ASSURED] mark=0 use=2`
	e, ok := parseConntrackLine(line)
	if !ok {
		t.Fatal("parse failed")
	}
	if e.proto != domain.ProtoTCP {
		t.Fatalf("proto %s", e.proto)
	}
	if e.src != "10.0.0.2" || e.dst != "1.1.1.1" {
		t.Fatalf("tuple %s -> %s", e.src, e.dst)
	}
	if e.sport != 45678 || e.dport != 443 {
		t.Fatalf("ports %d %d", e.sport, e.dport)
	}
	if e.bytes != 5000 {
		t.Fatalf("bytes %d", e.bytes)
	}
}

func TestParseConntrackLineUDP(t *testing.T) {
	line := `ipv4 2 udp 17 30 src=10.0.0.2 dst=8.8.8.8 sport=53 dport=53 packets=1 bytes=72 src=8.8.8.8 dst=10.0.0.2 sport=53 dport=53 packets=1 bytes=88`
	e, ok := parseConntrackLine(line)
	if !ok || e.proto != domain.ProtoUDP || e.bytes != 72 {
		t.Fatalf("%+v ok=%v", e, ok)
	}
}

func TestParseConntrackLineJunk(t *testing.T) {
	if _, ok := parseConntrackLine("not a conntrack line"); ok {
		t.Fatal("expected fail")
	}
	_ = strings.TrimSpace
}
