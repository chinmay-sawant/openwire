package wsl

import "testing"

func TestIsWSL2DoesNotPanic(t *testing.T) {
	// On this workspace we often run under WSL2; just ensure the helper is stable.
	_ = IsWSL2()
}
