//go:build !linux

package capture

import (
	"context"
	"fmt"
	"time"

	"github.com/chinmay-sawant/openwire/internal/domain"
)

// ProcStatsEngine is unavailable off Linux.
type ProcStatsEngine struct {
	Interval time.Duration
}

// Start always fails off Linux.
func (e *ProcStatsEngine) Start(ctx context.Context, ifaces []string) (<-chan domain.Observation, <-chan error, error) {
	return nil, nil, fmt.Errorf("%w: stats mode only supported on Linux", domain.ErrCaptureFailed)
}
