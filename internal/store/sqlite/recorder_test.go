package sqlite

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/chinmay-sawant/openwire/internal/domain"
	"github.com/chinmay-sawant/openwire/internal/store/memory"
)

func TestRecorderFlushAndLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")
	rec, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer rec.Close()

	mem := memory.New(memory.Config{MaxSamples: 60})
	now := time.Now().UTC().Truncate(time.Millisecond)
	// Build samples via ingest + tick.
	for i := 0; i < 5; i++ {
		mem.Ingest(domain.Observation{
			Time:      now.Add(time.Duration(i) * 200 * time.Millisecond),
			Iface:     "eth0",
			Direction: domain.DirectionRx,
			Length:    1000,
			Protocol:  domain.ProtoTCP,
			SrcIP:     "1.1.1.1",
			DstIP:     "10.0.0.2",
			SrcPort:   443,
			DstPort:   12345,
			AppHint:   "browser",
			PIDHint:   42,
		})
	}
	mem.TickSample(now.Add(2 * time.Second))
	mem.Ingest(domain.Observation{
		Time:      now.Add(2500 * time.Millisecond),
		Iface:     "eth0",
		Direction: domain.DirectionTx,
		Length:    500,
		AppHint:   "browser",
		PIDHint:   42,
	})
	mem.TickSample(now.Add(4 * time.Second))

	if err := rec.Flush(mem); err != nil {
		t.Fatal(err)
	}

	loaded, err := rec.LoadRecentSamples(100)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded) == 0 {
		t.Fatal("expected loaded samples")
	}

	// Reload into a fresh store.
	mem2 := memory.New(memory.Config{MaxSamples: 60})
	mem2.LoadSamples(loaded)
	if len(mem2.Samples()) == 0 {
		t.Fatal("memory should have historical samples")
	}
}

func TestOpenEmptyPath(t *testing.T) {
	if _, err := Open(""); err == nil {
		t.Fatal("expected error")
	}
}
