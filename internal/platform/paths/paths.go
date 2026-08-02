// Package paths resolves XDG-style state directories for OpenWire.
package paths

import (
	"os"
	"path/filepath"
)

// StateDir returns the durable state directory for logs and the default DB.
func StateDir() (string, error) {
	if xdg := os.Getenv("XDG_STATE_HOME"); xdg != "" {
		return filepath.Join(xdg, "openwire"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(os.TempDir(), "openwire"), nil
	}
	return filepath.Join(home, ".local", "state", "openwire"), nil
}

// DefaultDBPath is the default SQLite database file.
func DefaultDBPath() (string, error) {
	dir, err := StateDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "openwire.db"), nil
}

// EnsureStateDir creates the state directory if needed.
func EnsureStateDir() (string, error) {
	dir, err := StateDir()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}
