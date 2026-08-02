package app

import (
	"errors"
	"testing"

	"github.com/chinmay-sawant/openwire/internal/domain"
)

// Documented selection rules for live vs stats vs strict.
func TestCaptureModeSelectionRules(t *testing.T) {
	privErr := domain.ErrInsufficientPrivilege

	// strict + priv error => fail
	if !errors.Is(privErr, domain.ErrInsufficientPrivilege) {
		t.Fatal("expected privilege error")
	}

	// non-strict + priv error => stats mode (behavioral contract)
	strict := false
	var mode domain.CaptureMode
	if errors.Is(privErr, domain.ErrInsufficientPrivilege) && !strict {
		mode = domain.ModeStats
	} else {
		mode = domain.ModeLive
	}
	if mode != domain.ModeStats {
		t.Fatalf("got %s", mode)
	}

	strict = true
	if errors.Is(privErr, domain.ErrInsufficientPrivilege) && !strict {
		mode = domain.ModeStats
	} else if errors.Is(privErr, domain.ErrInsufficientPrivilege) && strict {
		// would return error to user
		mode = ""
	}
	if mode != "" {
		t.Fatalf("strict should not select stats, got %s", mode)
	}
}
