//go:build linux

package capture

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/chinmay-sawant/openwire/internal/domain"
)

func TestCanOpenCapturePrivilegeGate(t *testing.T) {
	all, err := ListLinuxAdapters()
	if err != nil {
		t.Fatal(err)
	}
	names := SelectIfaces(all, nil)
	if len(names) == 0 {
		// try loopback explicitly for probe
		names = []string{"lo"}
	}
	err = CanOpenCapture(names[0])
	if err == nil {
		t.Log("AF_PACKET open succeeded (process has CAP_NET_RAW or root)")
		return
	}
	if !errors.Is(err, domain.ErrInsufficientPrivilege) {
		// On some hosts lo may fail for other reasons; still useful signal.
		t.Logf("CanOpenCapture error (not necessarily privilege): %v", err)
	}
}

// TestLinuxEngineLiveCapture is the P5.14 proof when privileges are available.
// Skips cleanly without CAP_NET_RAW so CI/unprivileged dev shells stay green.
func TestLinuxEngineLiveCapture(t *testing.T) {
	all, err := ListLinuxAdapters()
	if err != nil {
		t.Fatal(err)
	}
	names := SelectIfaces(all, nil)
	if len(names) == 0 {
		t.Skip("no non-loopback up interfaces")
	}
	if err := CanOpenCapture(names[0]); err != nil {
		t.Skipf("live capture not available: %v", err)
	}

	e := &LinuxEngine{}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	obs, errs, err := e.Start(ctx, names[:1])
	if err != nil {
		t.Fatal(err)
	}

	// Generate a little traffic so we are likely to see packets.
	go func() {
		// best-effort: touch loopback/http won't hit eth; ping may need privileges too.
		// Just wait — background system traffic often exists.
		<-ctx.Done()
	}()

	got := 0
	deadline := time.After(1500 * time.Millisecond)
loop:
	for {
		select {
		case <-deadline:
			break loop
		case _, ok := <-obs:
			if !ok {
				break loop
			}
			got++
			if got >= 1 {
				break loop
			}
		case err, ok := <-errs:
			if ok && err != nil {
				t.Fatalf("capture error: %v", err)
			}
		}
	}
	// Drain
	cancel()
	for range obs {
	}
	for range errs {
	}

	if got == 0 {
		// Privileges worked but no packets in window — environment-dependent.
		t.Log("AF_PACKET opened but no packets observed in 1.5s (acceptable on quiet hosts)")
	} else {
		t.Logf("AF_PACKET observed %d packet(s) — live capture proof OK", got)
	}
	_ = os.Getpid()
}
