package app

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestPrintAdapters(t *testing.T) {
	var buf bytes.Buffer
	ctx, cancel := AdaptersContext(context.Background())
	defer cancel()
	if err := PrintAdapters(ctx, AdaptersOptions{Out: &buf}); err != nil {
		t.Fatal(err)
	}
	s := buf.String()
	if !strings.Contains(s, "SOURCE") || !strings.Contains(s, "NAME") {
		t.Fatalf("missing header: %s", s)
	}
	// At least loopback exists on Linux CI/dev.
	if !strings.Contains(s, "lo") && !strings.Contains(s, "linux") {
		t.Fatalf("expected linux adapters in output:\n%s", s)
	}
}

func TestHumanBytesAdapters(t *testing.T) {
	if humanBytes(500) != "500 B" {
		t.Fatalf("got %s", humanBytes(500))
	}
	if humanBytes(2048) != "2.0 KB" {
		t.Fatalf("got %s", humanBytes(2048))
	}
}
