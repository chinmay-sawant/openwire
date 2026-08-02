//go:build !linux

package capture

import (
	"context"
	"fmt"
	"time"

	"github.com/chinmay-sawant/openwire/internal/domain"
)

// DeepEngine is unavailable off Linux.
type DeepEngine struct {
	Interval time.Duration
	UseEBPF  bool
	Attr     *Attributor
}

// Start always fails off Linux.
func (e *DeepEngine) Start(ctx context.Context, ifaces []string) (<-chan domain.Observation, <-chan error, error) {
	return nil, nil, fmt.Errorf("%w: deep attribution only on Linux", domain.ErrCaptureFailed)
}

// ConntrackAvailable is always false off Linux.
func ConntrackAvailable() bool { return false }
