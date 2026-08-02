//go:build linux

package capture

import (
	"context"
	"testing"
	"time"
)

func TestReadProcNetDev(t *testing.T) {
	m, err := readProcNetDev()
	if err != nil {
		t.Fatal(err)
	}
	if len(m) == 0 {
		t.Fatal("expected interface counters")
	}
	if _, ok := m["lo"]; !ok {
		t.Fatal("expected lo in /proc/net/dev")
	}
}

func TestProcStatsEngineStartStop(t *testing.T) {
	e := &ProcStatsEngine{Interval: 50 * time.Millisecond}
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	obs, errs, err := e.Start(ctx, []string{"lo", "eth0"})
	if err != nil {
		t.Fatal(err)
	}
	// Drain until closed.
	for range obs {
	}
	for range errs {
	}
}
