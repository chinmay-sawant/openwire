//go:build !linux

package capture

import (
	"context"
	"fmt"

	"github.com/chinmay-sawant/openwire/internal/domain"
)

// LinuxEngine is unavailable on non-Linux builds.
type LinuxEngine struct{}

// Start always fails off Linux.
func (e *LinuxEngine) Start(ctx context.Context, ifaces []string) (<-chan domain.Observation, <-chan error, error) {
	return nil, nil, fmt.Errorf("%w: live capture only supported on Linux", domain.ErrCaptureFailed)
}

// CanOpenCapture always fails off Linux.
func CanOpenCapture(iface string) error {
	return fmt.Errorf("%w: live capture only supported on Linux", domain.ErrCaptureFailed)
}
