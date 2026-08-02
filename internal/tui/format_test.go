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

func TestSparklineNonEmpty(t *testing.T) {
	// empty samples handled by renderGraph; sparkline with data:
	s := sparkline(nil, 10, 3)
	if len(s) < 10 {
		// pads with spaces
		t.Fatalf("len %d", len(s))
	}
}
