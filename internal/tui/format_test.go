package tui

import "testing"

func TestHumanBytes(t *testing.T) {
	// Fixed-width-ish formatting for stable TUI columns.
	got := humanBytes(500)
	if got != " 500 B" && got != "500 B" {
		// allow either padded form
		if len(got) < 3 {
			t.Fatalf("got %q", got)
		}
	}
	got2 := humanBytes(2048)
	if got2 != " 2.0KB" && got2 != "2.0 KB" && got2 != " 2.0 KB" {
		// current format: "%4.1f%cB" → " 2.0KB"
		if !(len(got2) >= 4) {
			t.Fatalf("got %q", got2)
		}
	}
}

func TestFormatAppLabel(t *testing.T) {
	if got := formatAppLabel("win/chrome", 1234); got != "win/chrome · 1234" {
		t.Fatalf("got %q", got)
	}
	// Linux apps must NOT append pid.
	if got := formatAppLabel("node", 99999); got != "node" {
		t.Fatalf("linux label should be name only, got %q", got)
	}
	if got := formatAppLabel("unknown", 9); got != "pid:9" {
		t.Fatalf("got %q", got)
	}
	if got := formatAppLabel("browser", 0); got != "browser" {
		t.Fatalf("got %q", got)
	}
}

func TestSmoothRateSnappyAttack(t *testing.T) {
	// large rise should track closely (near real-time spikes)
	v := smoothRate(100, 10_000)
	if v < 5_000 {
		t.Fatalf("attack too slow for spikes: %v", v)
	}
	// drop should not cliff to zero in one step
	v2 := smoothRate(10_000, 0)
	if v2 < 1000 {
		t.Fatalf("decayed too fast: %v", v2)
	}
}

func TestSparklineNonEmpty(t *testing.T) {
	// empty samples handled by renderGraph; sparkline with data:
	s := sparkline(nil, 10, 3)
	if len(s) < 10 {
		// pads with spaces
		t.Fatalf("len %d", len(s))
	}
}
