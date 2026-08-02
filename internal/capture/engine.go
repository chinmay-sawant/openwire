// Package capture defines the packet/byte observation engines.
package capture

import (
	"context"

	"github.com/chinmay-sawant/openwire/internal/domain"
)

// Engine produces a stream of Observations until the context is cancelled.
type Engine interface {
	// Start begins capture on the given interface names (empty = engine default).
	// observations and errors channels are closed when the engine stops.
	Start(ctx context.Context, ifaces []string) (obs <-chan domain.Observation, errs <-chan error, err error)
}

// AdapterLister discovers local network adapters.
type AdapterLister interface {
	ListAdapters() ([]domain.Adapter, error)
}
