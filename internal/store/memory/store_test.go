package memory

import (
	"testing"
	"time"

	"github.com/chinmay-sawant/openwire/internal/domain"
)

func TestIngestAndRankApps(t *testing.T) {
	s := New(Config{MaxFlows: 100, MaxSamples: 60, MaxApps: 50, RateWindow: time.Second})
	now := time.Now()
	for i := 0; i < 10; i++ {
		s.Ingest(domain.Observation{
			Time:      now.Add(time.Duration(i) * 100 * time.Millisecond),
			Iface:     "eth0",
			Direction: domain.DirectionRx,
			Length:    1500,
			Protocol:  domain.ProtoTCP,
			SrcIP:     "1.1.1.1",
			DstIP:     "10.0.0.2",
			SrcPort:   443,
			DstPort:   45000,
			AppHint:   "browser",
			PIDHint:   100,
		})
	}
	for i := 0; i < 3; i++ {
		s.Ingest(domain.Observation{
			Time:      now.Add(time.Duration(i) * 100 * time.Millisecond),
			Iface:     "eth0",
			Direction: domain.DirectionTx,
			Length:    500,
			Protocol:  domain.ProtoTCP,
			SrcIP:     "10.0.0.2",
			DstIP:     "8.8.8.8",
			SrcPort:   50000,
			DstPort:   53,
			AppHint:   "dns",
			PIDHint:   200,
		})
	}
	// Force sample after 1s.
	s.TickSample(now.Add(2 * time.Second))

	apps := s.ListAppsByBandwidth(10)
	if len(apps) < 2 {
		t.Fatalf("expected >=2 apps, got %d", len(apps))
	}
	if apps[0].Name != "browser" {
		t.Fatalf("expected browser first, got %s", apps[0].Name)
	}
	samples := s.Samples()
	if len(samples) == 0 {
		t.Fatal("expected at least one bandwidth sample")
	}
	st := s.Snapshot()
	if st.TotalRxBytes == 0 {
		t.Fatal("expected rx bytes")
	}
}

func TestIfaceFilterApps(t *testing.T) {
	s := New(Config{MaxFlows: 100, MaxSamples: 60, MaxApps: 50})
	now := time.Now()
	s.Ingest(domain.Observation{
		Time: now, Iface: "eth0", Direction: domain.DirectionRx, Length: 1000,
		Protocol: domain.ProtoTCP, SrcIP: "1.1.1.1", DstIP: "10.0.0.2",
		SrcPort: 443, DstPort: 1, AppHint: "browser", PIDHint: 1,
	})
	s.Ingest(domain.Observation{
		Time: now, Iface: "wlan0", Direction: domain.DirectionRx, Length: 5000,
		Protocol: domain.ProtoTCP, SrcIP: "1.1.1.1", DstIP: "10.0.0.3",
		SrcPort: 443, DstPort: 2, AppHint: "browser", PIDHint: 1,
	})
	s.SetIfaceFilter("wlan0")
	apps := s.ListAppsByBandwidth(10)
	if len(apps) != 1 {
		t.Fatalf("want 1 app, got %d", len(apps))
	}
	if apps[0].BytesIn != 5000 {
		t.Fatalf("filtered bytes %d", apps[0].BytesIn)
	}
	s.SetIfaceFilter("")
	apps = s.ListAppsByBandwidth(10)
	if apps[0].BytesIn != 6000 {
		t.Fatalf("all bytes %d", apps[0].BytesIn)
	}
}

func TestEvictRespectsMaxFlows(t *testing.T) {
	s := New(Config{MaxFlows: 5, MaxSamples: 10, MaxApps: 20})
	now := time.Now()
	for i := 0; i < 20; i++ {
		s.Ingest(domain.Observation{
			Time:     now.Add(time.Duration(i) * time.Millisecond),
			Iface:    "eth0",
			Length:   100,
			Protocol: domain.ProtoUDP,
			SrcIP:    "10.0.0.1",
			DstIP:    "10.0.0.2",
			SrcPort:  uint16(1000 + i),
			DstPort:  53,
			AppHint:  "app",
		})
	}
	st := s.Snapshot()
	if st.FlowCount > 5 {
		t.Fatalf("flow count %d exceeds max 5", st.FlowCount)
	}
}
