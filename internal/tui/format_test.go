package tui

import "testing"

func TestHumanBytes(t *testing.T) {
	if humanBytes(500) != "500 B" {
		t.Fatalf("got %s", humanBytes(500))
	}
	if humanBytes(2048) != "2.0 KB" {
		t.Fatalf("got %s", humanBytes(2048))
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
