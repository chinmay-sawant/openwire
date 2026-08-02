package capture

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/chinmay-sawant/openwire/internal/domain"
)

// DemoEngine emits synthetic multi-app traffic for UI development.
type DemoEngine struct {
	Interval time.Duration
	RNG      *rand.Rand
}

type demoApp struct {
	name   string
	pid    int
	weight int
}

var demoApps = []demoApp{
	{name: "browser", pid: 1200, weight: 8},
	{name: "code", pid: 1300, weight: 4},
	{name: "ssh", pid: 1400, weight: 2},
	{name: "dns", pid: 1500, weight: 1},
	{name: "unknown", pid: 0, weight: 1},
}

// Start implements Engine.
func (d *DemoEngine) Start(ctx context.Context, ifaces []string) (<-chan domain.Observation, <-chan error, error) {
	interval := d.Interval
	if interval <= 0 {
		interval = 100 * time.Millisecond
	}
	rng := d.RNG
	if rng == nil {
		rng = rand.New(rand.NewSource(time.Now().UnixNano()))
	}
	iface := "eth0"
	if len(ifaces) > 0 {
		iface = ifaces[0]
	}

	obs := make(chan domain.Observation, 256)
	errs := make(chan error, 1)

	go func() {
		defer close(obs)
		defer close(errs)
		t := time.NewTicker(interval)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case now := <-t.C:
				n := 1 + rng.Intn(5)
				for i := 0; i < n; i++ {
					app := pickApp(rng)
					dir := domain.DirectionRx
					if rng.Intn(2) == 0 {
						dir = domain.DirectionTx
					}
					o := domain.Observation{
						Time:      now,
						Iface:     iface,
						Direction: dir,
						Length:    200 + rng.Intn(1400),
						Protocol:  domain.ProtoTCP,
						SrcIP:     "10.0.0.2",
						DstIP:     fmt.Sprintf("%d.%d.%d.%d", rng.Intn(223)+1, rng.Intn(255), rng.Intn(255), rng.Intn(254)+1),
						SrcPort:   uint16(40000 + rng.Intn(20000)),
						DstPort:   443,
						AppHint:   app.name,
						PIDHint:   app.pid,
					}
					if app.name == "dns" {
						o.Protocol = domain.ProtoUDP
						o.DstPort = 53
						o.Length = 64 + rng.Intn(200)
					}
					select {
					case obs <- o:
					case <-ctx.Done():
						return
					default:
					}
				}
			}
		}
	}()
	return obs, errs, nil
}

func pickApp(rng *rand.Rand) demoApp {
	total := 0
	for _, a := range demoApps {
		total += a.weight
	}
	r := rng.Intn(total)
	for _, a := range demoApps {
		r -= a.weight
		if r < 0 {
			return a
		}
	}
	return demoApps[0]
}

// DemoAdapters returns a fixed adapter inventory for demo mode.
func DemoAdapters() []domain.Adapter {
	return []domain.Adapter{
		{
			Name:     "eth0",
			Index:    2,
			Hardware: "00:11:22:33:44:55",
			IPv4:     []string{"10.0.0.2"},
			Up:       true,
			Source:   domain.AdapterSourceDemo,
		},
		{
			Name:     "lo",
			Index:    1,
			IPv4:     []string{"127.0.0.1"},
			Up:       true,
			Loopback: true,
			Source:   domain.AdapterSourceDemo,
		},
	}
}
