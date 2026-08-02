// Package logging configures file-based logs so the TUI is not corrupted.
package logging

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/chinmay-sawant/openwire/internal/platform/paths"
)

// Setup installs a default slog logger writing to a state-dir log file.
// Returns the log path (may be empty if falling back to discard).
func Setup(level string) (logPath string, closer io.Closer, err error) {
	dir, err := paths.EnsureStateDir()
	if err != nil {
		slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
		return "", nopCloser{}, nil
	}
	path := filepath.Join(dir, "openwire.log")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
		return "", nopCloser{}, nil
	}
	lvl := parseLevel(level)
	slog.SetDefault(slog.New(slog.NewTextHandler(f, &slog.HandlerOptions{Level: lvl})))
	return path, f, nil
}

func parseLevel(s string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

type nopCloser struct{}

func (nopCloser) Close() error { return nil }

// PathHint returns a short user-facing note about where logs go.
func PathHint(path string) string {
	if path == "" {
		return "logging disabled"
	}
	return fmt.Sprintf("logs: %s", path)
}
