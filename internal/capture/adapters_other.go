//go:build !linux

package capture

import (
	"fmt"

	"github.com/chinmay-sawant/openwire/internal/domain"
)

// ListLinuxAdapters is unavailable on non-Linux.
func ListLinuxAdapters() ([]domain.Adapter, error) {
	return nil, fmt.Errorf("adapter discovery is only implemented on Linux")
}

// SelectIfaces returns the requested names unchanged on non-Linux.
func SelectIfaces(all []domain.Adapter, want []string) []string {
	if len(want) > 0 {
		return append([]string(nil), want...)
	}
	return nil
}
