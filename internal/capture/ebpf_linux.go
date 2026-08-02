//go:build linux

package capture

import (
	"context"
	"encoding/binary"
	"fmt"
	"log/slog"
	"os"
	"runtime"
	"strconv"
	"strings"
	"unsafe"

	"golang.org/x/sys/unix"
)

// EBPFCounter is a best-effort per-PID counter fed by a kprobe BPF program on
// tcp_sendmsg. Requires privileges (CAP_BPF/CAP_PERFMON/root) and tracefs.
// Counts send events per PID (not exact byte totals); DeepEngine still uses
// conntrack for byte-accurate flow accounting when available.
type EBPFCounter struct {
	mapFD   int
	progFD  int
	perfFD  int
	evName  string
	tracefs string
}

// StartEBPFCounters tries to load and attach the kprobe program.
func StartEBPFCounters(ctx context.Context) (*EBPFCounter, error) {
	if runtime.GOARCH != "amd64" {
		return nil, fmt.Errorf("ebpf: kprobe program supported on amd64 only (have %s)", runtime.GOARCH)
	}

	mapFD, err := bpfCreateMap(unix.BPF_MAP_TYPE_HASH, 4, 8, 10240)
	if err != nil {
		return nil, fmt.Errorf("ebpf map: %w", err)
	}

	insns := buildKprobeEventCountProgram(mapFD)
	progFD, err := bpfLoadProgram(unix.BPF_PROG_TYPE_KPROBE, insns, "GPL")
	if err != nil {
		_ = unix.Close(mapFD)
		return nil, fmt.Errorf("ebpf load: %w", err)
	}

	tracefs, err := findTracefs()
	if err != nil {
		_ = unix.Close(progFD)
		_ = unix.Close(mapFD)
		return nil, err
	}
	evName := "ow_tcp_sendmsg"
	_ = removeKprobe(tracefs, evName)
	if err := addKprobe(tracefs, evName, "tcp_sendmsg"); err != nil {
		_ = unix.Close(progFD)
		_ = unix.Close(mapFD)
		return nil, err
	}
	id, err := readTraceEventID(tracefs, "kprobes", evName)
	if err != nil {
		_ = removeKprobe(tracefs, evName)
		_ = unix.Close(progFD)
		_ = unix.Close(mapFD)
		return nil, err
	}

	attr := unix.PerfEventAttr{
		Type:   unix.PERF_TYPE_TRACEPOINT,
		Size:   uint32(unsafe.Sizeof(unix.PerfEventAttr{})),
		Config: uint64(id),
		Sample: 1,
		Wakeup: 1,
	}
	pfd, err := unix.PerfEventOpen(&attr, -1, 0, -1, unix.PERF_FLAG_FD_CLOEXEC)
	if err != nil {
		_ = removeKprobe(tracefs, evName)
		_ = unix.Close(progFD)
		_ = unix.Close(mapFD)
		return nil, fmt.Errorf("perf_event_open: %w", err)
	}
	if err := unix.IoctlSetInt(pfd, unix.PERF_EVENT_IOC_SET_BPF, progFD); err != nil {
		_ = unix.Close(pfd)
		_ = removeKprobe(tracefs, evName)
		_ = unix.Close(progFD)
		_ = unix.Close(mapFD)
		return nil, fmt.Errorf("PERF_EVENT_IOC_SET_BPF: %w", err)
	}
	if err := unix.IoctlSetInt(pfd, unix.PERF_EVENT_IOC_ENABLE, 0); err != nil {
		_ = unix.Close(pfd)
		_ = removeKprobe(tracefs, evName)
		_ = unix.Close(progFD)
		_ = unix.Close(mapFD)
		return nil, fmt.Errorf("PERF_EVENT_IOC_ENABLE: %w", err)
	}

	c := &EBPFCounter{
		mapFD:   mapFD,
		progFD:  progFD,
		perfFD:  pfd,
		evName:  evName,
		tracefs: tracefs,
	}
	go func() {
		<-ctx.Done()
		_ = c.Close()
	}()
	slog.Debug("ebpf counters attached", "event", evName, "id", id)
	return c, nil
}

// Snapshot returns pid → cumulative event counts.
func (c *EBPFCounter) Snapshot() (map[uint32]uint64, error) {
	out := make(map[uint32]uint64)
	if c == nil || c.mapFD <= 0 {
		return out, fmt.Errorf("ebpf: closed")
	}
	var key, nextKey [4]byte
	first := true
	for {
		var err error
		if first {
			err = bpfMapGetNextKey(c.mapFD, nil, nextKey[:])
			first = false
		} else {
			copy(key[:], nextKey[:])
			err = bpfMapGetNextKey(c.mapFD, key[:], nextKey[:])
		}
		if err != nil {
			break
		}
		var val [8]byte
		if err := bpfMapLookup(c.mapFD, nextKey[:], val[:]); err != nil {
			continue
		}
		pid := binary.LittleEndian.Uint32(nextKey[:])
		out[pid] = binary.LittleEndian.Uint64(val[:])
	}
	return out, nil
}

// Close detaches the kprobe and closes fds.
func (c *EBPFCounter) Close() error {
	if c == nil {
		return nil
	}
	if c.perfFD > 0 {
		_ = unix.IoctlSetInt(c.perfFD, unix.PERF_EVENT_IOC_DISABLE, 0)
		_ = unix.Close(c.perfFD)
		c.perfFD = 0
	}
	if c.tracefs != "" && c.evName != "" {
		_ = removeKprobe(c.tracefs, c.evName)
	}
	if c.progFD > 0 {
		_ = unix.Close(c.progFD)
		c.progFD = 0
	}
	if c.mapFD > 0 {
		_ = unix.Close(c.mapFD)
		c.mapFD = 0
	}
	return nil
}

func findTracefs() (string, error) {
	for _, t := range []string{"/sys/kernel/tracing", "/sys/kernel/debug/tracing"} {
		if st, err := os.Stat(t); err == nil && st.IsDir() {
			return t, nil
		}
	}
	return "", fmt.Errorf("tracefs not mounted")
}

func addKprobe(tracefs, name, symbol string) error {
	// p:name symbol
	return os.WriteFile(tracefs+"/kprobe_events", []byte("p:"+name+" "+symbol+"\n"), 0o644)
}

func removeKprobe(tracefs, name string) error {
	return os.WriteFile(tracefs+"/kprobe_events", []byte("-:"+name+"\n"), 0o644)
}

func readTraceEventID(tracefs, group, name string) (int, error) {
	b, err := os.ReadFile(tracefs + "/events/" + group + "/" + name + "/id")
	if err != nil {
		return 0, err
	}
	s := strings.TrimSpace(string(b))
	id, err := strconv.Atoi(s)
	if err != nil || id <= 0 {
		return 0, fmt.Errorf("bad event id %q", s)
	}
	return id, nil
}

// --- bpf(2) wrappers ---

type bpfInsn struct {
	Code uint8
	Regs uint8
	Off  int16
	Imm  int32
}

func reg(dst, src uint8) uint8 { return (dst & 0xf) | ((src & 0xf) << 4) }

func bpfSys(cmd int, attr []byte) (int, error) {
	r1, _, errno := unix.Syscall(unix.SYS_BPF, uintptr(cmd), uintptr(unsafe.Pointer(&attr[0])), uintptr(len(attr)))
	if errno != 0 {
		return -1, errno
	}
	return int(r1), nil
}

func bpfCreateMap(typ int, keySize, valueSize, maxEntries uint32) (int, error) {
	attr := make([]byte, 128)
	binary.LittleEndian.PutUint32(attr[0:4], uint32(typ))
	binary.LittleEndian.PutUint32(attr[4:8], keySize)
	binary.LittleEndian.PutUint32(attr[8:12], valueSize)
	binary.LittleEndian.PutUint32(attr[12:16], maxEntries)
	return bpfSys(unix.BPF_MAP_CREATE, attr)
}

func bpfLoadProgram(typ int, insns []bpfInsn, license string) (int, error) {
	lic := append([]byte(license), 0)
	logBuf := make([]byte, 1<<16)
	attr := make([]byte, 128)
	binary.LittleEndian.PutUint32(attr[0:4], uint32(typ))
	binary.LittleEndian.PutUint32(attr[4:8], uint32(len(insns)))
	binary.LittleEndian.PutUint64(attr[8:16], uint64(uintptr(unsafe.Pointer(&insns[0]))))
	binary.LittleEndian.PutUint64(attr[16:24], uint64(uintptr(unsafe.Pointer(&lic[0]))))
	binary.LittleEndian.PutUint32(attr[24:28], 1) // log_level
	binary.LittleEndian.PutUint32(attr[28:32], uint32(len(logBuf)))
	binary.LittleEndian.PutUint64(attr[32:40], uint64(uintptr(unsafe.Pointer(&logBuf[0]))))
	// kern_version left 0
	fd, err := bpfSys(unix.BPF_PROG_LOAD, attr)
	if err != nil {
		msg := string(logBuf)
		if i := strings.IndexByte(msg, 0); i >= 0 {
			msg = msg[:i]
		}
		msg = strings.TrimSpace(msg)
		if msg != "" {
			return -1, fmt.Errorf("%w: %s", err, msg)
		}
		return -1, err
	}
	return fd, nil
}

func bpfMapGetNextKey(mapFD int, key, nextKey []byte) error {
	attr := make([]byte, 128)
	binary.LittleEndian.PutUint32(attr[0:4], uint32(mapFD))
	if key != nil {
		binary.LittleEndian.PutUint64(attr[8:16], uint64(uintptr(unsafe.Pointer(&key[0]))))
	}
	binary.LittleEndian.PutUint64(attr[16:24], uint64(uintptr(unsafe.Pointer(&nextKey[0]))))
	_, err := bpfSys(unix.BPF_MAP_GET_NEXT_KEY, attr)
	return err
}

func bpfMapLookup(mapFD int, key, value []byte) error {
	attr := make([]byte, 128)
	binary.LittleEndian.PutUint32(attr[0:4], uint32(mapFD))
	binary.LittleEndian.PutUint64(attr[8:16], uint64(uintptr(unsafe.Pointer(&key[0]))))
	binary.LittleEndian.PutUint64(attr[16:24], uint64(uintptr(unsafe.Pointer(&value[0]))))
	_, err := bpfSys(unix.BPF_MAP_LOOKUP_ELEM, attr)
	return err
}

// buildKprobeEventCountProgram increments map[pid] on each kprobe hit.
func buildKprobeEventCountProgram(mapFD int) []bpfInsn {
	const (
		BPF_LD    = 0x00
		BPF_LDX   = 0x01
		BPF_STX   = 0x03
		BPF_ALU64 = 0x07
		BPF_JMP   = 0x05
		BPF_MOV   = 0xb0
		BPF_ADD   = 0x00
		BPF_RSH   = 0x70
		BPF_JEQ   = 0x10
		BPF_EXIT  = 0x90
		BPF_CALL  = 0x80
		BPF_IMM   = 0x00
		BPF_DW    = 0x18
		BPF_W     = 0x00
		BPF_MEM   = 0x60
		BPF_X     = 0x08
		BPF_K     = 0x00
	)
	const (
		fnLookup = 1
		fnUpdate = 2
		fnPid    = 14
	)
	// BPF_PSEUDO_MAP_FD = 1 in src register for lddw
	return []bpfInsn{
		// r0 = bpf_get_current_pid_tgid()
		{Code: BPF_JMP | BPF_CALL, Imm: fnPid},
		// r6 = pid = r0 >> 32
		{Code: BPF_ALU64 | BPF_MOV | BPF_X, Regs: reg(6, 0)},
		{Code: BPF_ALU64 | BPF_RSH | BPF_K, Regs: reg(6, 0), Imm: 32},
		// *(u32 *)(r10 - 4) = r6
		{Code: BPF_STX | BPF_MEM | BPF_W, Regs: reg(10, 6), Off: -4},
		// r1 = map fd (lddw)
		{Code: BPF_LD | BPF_DW | BPF_IMM, Regs: reg(1, 1), Imm: int32(mapFD)},
		{Code: 0, Imm: 0},
		// r2 = r10 - 4
		{Code: BPF_ALU64 | BPF_MOV | BPF_X, Regs: reg(2, 10)},
		{Code: BPF_ALU64 | BPF_ADD | BPF_K, Regs: reg(2, 0), Imm: -4},
		// r0 = map_lookup_elem
		{Code: BPF_JMP | BPF_CALL, Imm: fnLookup},
		// if r0 == 0 goto +5 (init path)
		{Code: BPF_JMP | BPF_JEQ | BPF_K, Regs: reg(0, 0), Off: 5, Imm: 0},
		// r1 = *(u64 *)(r0 + 0)
		{Code: BPF_LDX | BPF_MEM | BPF_DW, Regs: reg(1, 0), Off: 0},
		// r1 += 1
		{Code: BPF_ALU64 | BPF_ADD | BPF_K, Regs: reg(1, 0), Imm: 1},
		// *(u64 *)(r0 + 0) = r1
		{Code: BPF_STX | BPF_MEM | BPF_DW, Regs: reg(0, 1), Off: 0},
		// r0 = 0; exit
		{Code: BPF_ALU64 | BPF_MOV | BPF_K, Regs: reg(0, 0), Imm: 0},
		{Code: BPF_JMP | BPF_EXIT},
		// init: *(u64 *)(r10-16) = 1
		{Code: BPF_ALU64 | BPF_MOV | BPF_K, Regs: reg(1, 0), Imm: 1},
		{Code: BPF_STX | BPF_MEM | BPF_DW, Regs: reg(10, 1), Off: -16},
		// r1 = map fd
		{Code: BPF_LD | BPF_DW | BPF_IMM, Regs: reg(1, 1), Imm: int32(mapFD)},
		{Code: 0, Imm: 0},
		// r2 = &pid
		{Code: BPF_ALU64 | BPF_MOV | BPF_X, Regs: reg(2, 10)},
		{Code: BPF_ALU64 | BPF_ADD | BPF_K, Regs: reg(2, 0), Imm: -4},
		// r3 = &val
		{Code: BPF_ALU64 | BPF_MOV | BPF_X, Regs: reg(3, 10)},
		{Code: BPF_ALU64 | BPF_ADD | BPF_K, Regs: reg(3, 0), Imm: -16},
		// r4 = BPF_ANY
		{Code: BPF_ALU64 | BPF_MOV | BPF_K, Regs: reg(4, 0), Imm: 0},
		// map_update_elem
		{Code: BPF_JMP | BPF_CALL, Imm: fnUpdate},
		{Code: BPF_ALU64 | BPF_MOV | BPF_K, Regs: reg(0, 0), Imm: 0},
		{Code: BPF_JMP | BPF_EXIT},
	}
}
