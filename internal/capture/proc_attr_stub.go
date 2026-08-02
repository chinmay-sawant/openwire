//go:build !linux

package capture

import (
	"fmt"
	"time"

	"github.com/chinmay-sawant/openwire/internal/domain"
)

// Attributor is a no-op off Linux.
type Attributor struct{}

// NewAttributor returns a no-op attributor.
func NewAttributor(every time.Duration) *Attributor {
	return &Attributor{}
}

// Run is a no-op.
func (a *Attributor) Run(done <-chan struct{}) {}

// Annotate is a no-op.
func (a *Attributor) Annotate(o *domain.Observation) {}

// ProcessName is unavailable off Linux.
func ProcessName(pid int) string { return fmt.Sprintf("pid:%d", pid) }
