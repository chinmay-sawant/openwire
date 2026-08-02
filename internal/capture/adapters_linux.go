//go:build linux

package capture

import (
	"net"

	"github.com/chinmay-sawant/openwire/internal/domain"
)

// ListLinuxAdapters returns host network interfaces (Linux).
func ListLinuxAdapters() ([]domain.Adapter, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	out := make([]domain.Adapter, 0, len(ifaces))
	for _, iface := range ifaces {
		a := domain.Adapter{
			Name:     iface.Name,
			Index:    iface.Index,
			Hardware: iface.HardwareAddr.String(),
			Up:       iface.Flags&net.FlagUp != 0,
			Loopback: iface.Flags&net.FlagLoopback != 0,
			Source:   domain.AdapterSourceLinux,
		}
		addrs, err := iface.Addrs()
		if err == nil {
			for _, addr := range addrs {
				switch v := addr.(type) {
				case *net.IPNet:
					if ip4 := v.IP.To4(); ip4 != nil {
						a.IPv4 = append(a.IPv4, ip4.String())
					} else if v.IP.To16() != nil {
						a.IPv6 = append(a.IPv6, v.IP.String())
					}
				case *net.IPAddr:
					if ip4 := v.IP.To4(); ip4 != nil {
						a.IPv4 = append(a.IPv4, ip4.String())
					} else if v.IP.To16() != nil {
						a.IPv6 = append(a.IPv6, v.IP.String())
					}
				}
			}
		}
		out = append(out, a)
	}
	return out, nil
}

// SelectIfaces picks capture targets: non-loopback up interfaces, or explicit list.
func SelectIfaces(all []domain.Adapter, want []string) []string {
	if len(want) > 0 {
		return append([]string(nil), want...)
	}
	var names []string
	for _, a := range all {
		if a.Source != domain.AdapterSourceLinux {
			continue
		}
		if a.Loopback || !a.Up {
			continue
		}
		names = append(names, a.Name)
	}
	// Fall back to loopback if nothing else (useful in constrained envs).
	if len(names) == 0 {
		for _, a := range all {
			if a.Source == domain.AdapterSourceLinux && a.Up {
				names = append(names, a.Name)
			}
		}
	}
	return names
}
