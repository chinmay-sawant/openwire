//go:build linux

package capture

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"sync"
	"syscall"
	"time"

	"github.com/chinmay-sawant/openwire/internal/domain"
	"golang.org/x/sys/unix"
)

// LinuxEngine captures using AF_PACKET raw sockets (no libpcap/CGO).
type LinuxEngine struct {
	LocalNets []*net.IPNet
}

// Start implements Engine. Requires CAP_NET_RAW or root.
func (e *LinuxEngine) Start(ctx context.Context, ifaces []string) (<-chan domain.Observation, <-chan error, error) {
	if len(ifaces) == 0 {
		return nil, nil, domain.ErrNoAdapters
	}

	// Probe privilege on first iface before spawning workers.
	if err := CanOpenCapture(ifaces[0]); err != nil {
		return nil, nil, err
	}

	local := e.LocalNets
	if len(local) == 0 {
		local = collectLocalNets()
	}

	obs := make(chan domain.Observation, 1024)
	errs := make(chan error, 4)

	var wg sync.WaitGroup
	for _, name := range ifaces {
		name := name
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := captureIface(ctx, name, local, obs); err != nil && ctx.Err() == nil {
				select {
				case errs <- fmt.Errorf("%s: %w", name, err):
				default:
				}
			}
		}()
	}

	go func() {
		wg.Wait()
		close(obs)
		close(errs)
	}()

	return obs, errs, nil
}

func openAFPacket(iface string) (int, error) {
	fd, err := unix.Socket(unix.AF_PACKET, unix.SOCK_RAW, int(htons(unix.ETH_P_ALL)))
	if err != nil {
		return -1, err
	}
	ifi, err := net.InterfaceByName(iface)
	if err != nil {
		_ = unix.Close(fd)
		return -1, err
	}
	addr := unix.SockaddrLinklayer{
		Protocol: htons(unix.ETH_P_ALL),
		Ifindex:  ifi.Index,
	}
	if err := unix.Bind(fd, &addr); err != nil {
		_ = unix.Close(fd)
		return -1, err
	}
	return fd, nil
}

func captureIface(ctx context.Context, iface string, local []*net.IPNet, out chan<- domain.Observation) error {
	fd, err := openAFPacket(iface)
	if err != nil {
		if isPerm(err) {
			return fmt.Errorf("%w: %v", domain.ErrInsufficientPrivilege, err)
		}
		return err
	}
	// Ensure close once; context cancel unblocks Recvfrom.
	var once sync.Once
	closeFD := func() { once.Do(func() { _ = unix.Close(fd) }) }
	defer closeFD()
	go func() {
		<-ctx.Done()
		closeFD()
	}()

	buf := make([]byte, 65536)
	for {
		if ctx.Err() != nil {
			return nil
		}
		n, _, err := unix.Recvfrom(fd, buf, 0)
		if err != nil {
			if ctx.Err() != nil || errors.Is(err, unix.EBADF) || errors.Is(err, unix.EINTR) {
				return nil
			}
			if errors.Is(err, syscall.EAGAIN) {
				continue
			}
			return err
		}
		if n < 14 {
			continue
		}
		o, ok := parseEthernet(buf[:n], iface, local)
		if !ok {
			continue
		}
		o.Time = time.Now()
		select {
		case out <- o:
		case <-ctx.Done():
			return nil
		default:
		}
	}
}

func parseEthernet(frame []byte, iface string, local []*net.IPNet) (domain.Observation, bool) {
	var o domain.Observation
	o.Iface = iface
	o.Length = len(frame)
	if len(frame) < 14 {
		return o, false
	}
	ethType := binary.BigEndian.Uint16(frame[12:14])
	payload := frame[14:]
	switch ethType {
	case 0x0800:
		return parseIPv4(payload, o, local)
	case 0x86DD:
		return parseIPv6(payload, o, local)
	default:
		o.Protocol = domain.ProtoOther
		o.Direction = domain.DirectionUnknown
		return o, true
	}
}

func parseIPv4(b []byte, o domain.Observation, local []*net.IPNet) (domain.Observation, bool) {
	if len(b) < 20 {
		return o, false
	}
	ihl := int(b[0]&0x0f) * 4
	if ihl < 20 || len(b) < ihl {
		return o, false
	}
	totalLen := int(binary.BigEndian.Uint16(b[2:4]))
	if totalLen > 0 {
		o.Length = 14 + totalLen
	}
	proto := b[9]
	src := net.IPv4(b[12], b[13], b[14], b[15])
	dst := net.IPv4(b[16], b[17], b[18], b[19])
	o.SrcIP = src.String()
	o.DstIP = dst.String()
	o.Direction = guessDirection(src, dst, local)
	rest := b[ihl:]
	switch proto {
	case 6:
		o.Protocol = domain.ProtoTCP
		if len(rest) >= 4 {
			o.SrcPort = binary.BigEndian.Uint16(rest[0:2])
			o.DstPort = binary.BigEndian.Uint16(rest[2:4])
		}
	case 17:
		o.Protocol = domain.ProtoUDP
		if len(rest) >= 4 {
			o.SrcPort = binary.BigEndian.Uint16(rest[0:2])
			o.DstPort = binary.BigEndian.Uint16(rest[2:4])
		}
	case 1:
		o.Protocol = domain.ProtoICMP
	default:
		o.Protocol = domain.ProtoOther
	}
	return o, true
}

func parseIPv6(b []byte, o domain.Observation, local []*net.IPNet) (domain.Observation, bool) {
	if len(b) < 40 {
		return o, false
	}
	next := b[6]
	src := net.IP(b[8:24])
	dst := net.IP(b[24:40])
	o.SrcIP = src.String()
	o.DstIP = dst.String()
	o.Direction = guessDirection(src, dst, local)
	rest := b[40:]
	for i := 0; i < 4; i++ {
		switch next {
		case 6:
			o.Protocol = domain.ProtoTCP
			if len(rest) >= 4 {
				o.SrcPort = binary.BigEndian.Uint16(rest[0:2])
				o.DstPort = binary.BigEndian.Uint16(rest[2:4])
			}
			return o, true
		case 17:
			o.Protocol = domain.ProtoUDP
			if len(rest) >= 4 {
				o.SrcPort = binary.BigEndian.Uint16(rest[0:2])
				o.DstPort = binary.BigEndian.Uint16(rest[2:4])
			}
			return o, true
		case 58:
			o.Protocol = domain.ProtoICMP
			return o, true
		case 0, 43, 60:
			if len(rest) < 2 {
				o.Protocol = domain.ProtoOther
				return o, true
			}
			next = rest[0]
			hdrLen := int(rest[1]+1) * 8
			if hdrLen <= 0 || len(rest) < hdrLen {
				o.Protocol = domain.ProtoOther
				return o, true
			}
			rest = rest[hdrLen:]
		case 44:
			if len(rest) < 8 {
				o.Protocol = domain.ProtoOther
				return o, true
			}
			next = rest[0]
			rest = rest[8:]
		default:
			o.Protocol = domain.ProtoOther
			return o, true
		}
	}
	o.Protocol = domain.ProtoOther
	return o, true
}

func guessDirection(src, dst net.IP, local []*net.IPNet) domain.Direction {
	srcLocal := ipInNets(src, local)
	dstLocal := ipInNets(dst, local)
	switch {
	case srcLocal && !dstLocal:
		return domain.DirectionTx
	case !srcLocal && dstLocal:
		return domain.DirectionRx
	default:
		return domain.DirectionUnknown
	}
}

func ipInNets(ip net.IP, nets []*net.IPNet) bool {
	for _, n := range nets {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}

func collectLocalNets() []*net.IPNet {
	var nets []*net.IPNet
	ifaces, err := net.Interfaces()
	if err != nil {
		return nets
	}
	for _, iface := range ifaces {
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, a := range addrs {
			if ipn, ok := a.(*net.IPNet); ok {
				nets = append(nets, ipn)
			}
		}
	}
	return nets
}

func htons(v uint16) uint16 {
	var b [2]byte
	binary.BigEndian.PutUint16(b[:], v)
	return binary.LittleEndian.Uint16(b[:])
}

func isPerm(err error) bool {
	return errors.Is(err, unix.EPERM) || errors.Is(err, unix.EACCES) || os.IsPermission(err)
}

// CanOpenCapture probes whether AF_PACKET can be opened.
func CanOpenCapture(iface string) error {
	if iface == "" {
		all, err := ListLinuxAdapters()
		if err != nil {
			return err
		}
		names := SelectIfaces(all, nil)
		if len(names) == 0 {
			return domain.ErrNoAdapters
		}
		iface = names[0]
	}
	fd, err := openAFPacket(iface)
	if err != nil {
		if isPerm(err) {
			return fmt.Errorf("%w: %v", domain.ErrInsufficientPrivilege, err)
		}
		return err
	}
	_ = unix.Close(fd)
	slog.Debug("capture probe ok", "iface", iface)
	return nil
}
