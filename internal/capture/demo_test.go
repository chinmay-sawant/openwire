package capture

import (
	"context"
	"testing"
	"time"
)

func TestDemoEngineEmitsObservations(t *testing.T) {
	e := &DemoEngine{Interval: 20 * time.Millisecond}
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()

	obs, errs, err := e.Start(ctx, []string{"eth0"})
	if err != nil {
		t.Fatal(err)
	}

	count := 0
	for o := range obs {
		count++
		if o.Length <= 0 {
			t.Fatalf("bad length: %+v", o)
		}
		if o.AppHint == "" {
			t.Fatalf("expected app hint: %+v", o)
		}
	}
	// drain errors
	for range errs {
	}
	if count == 0 {
		t.Fatal("expected demo observations")
	}
}

func TestDemoAdaptersNonEmpty(t *testing.T) {
	ads := DemoAdapters()
	if len(ads) == 0 {
		t.Fatal("expected demo adapters")
	}
}
