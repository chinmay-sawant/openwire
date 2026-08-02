//go:build !linux

package capture

import (
	"context"
	"fmt"
)

// EBPFCounter is unavailable off Linux.
type EBPFCounter struct{}

// StartEBPFCounters always fails off Linux.
func StartEBPFCounters(ctx context.Context) (*EBPFCounter, error) {
	return nil, fmt.Errorf("ebpf: only on linux")
}

// Snapshot returns an error off Linux.
func (c *EBPFCounter) Snapshot() (map[uint32]uint64, error) {
	return nil, fmt.Errorf("ebpf: only on linux")
}

// Close is a no-op.
func (c *EBPFCounter) Close() error { return nil }
