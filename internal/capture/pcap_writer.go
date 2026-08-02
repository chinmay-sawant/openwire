package capture

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Link type Ethernet (10/100Mb) for AF_PACKET frames.
const pcapLinkTypeEthernet = 1

// PCAPWriter writes classic libpcap-format captures (readable by Wireshark).
// Thread-safe. Rotates to path.1 when MaxBytes is exceeded (single backup).
type PCAPWriter struct {
	mu       sync.Mutex
	path     string
	maxBytes int64
	f        *os.File
	written  int64
	packets  uint64
}

// NewPCAPWriter creates the file and writes the global header.
// maxBytes <= 0 means no rotation (unbounded).
func NewPCAPWriter(path string, maxBytes int64) (*PCAPWriter, error) {
	if path == "" {
		return nil, fmt.Errorf("pcap: empty path")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		// If dir is "." mkdir may fail on empty; ignore when path has no dir.
		if filepath.Dir(path) != "." {
			return nil, err
		}
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, err
	}
	w := &PCAPWriter{path: path, maxBytes: maxBytes, f: f}
	if err := w.writeGlobalHeader(); err != nil {
		_ = f.Close()
		return nil, err
	}
	return w, nil
}

func (w *PCAPWriter) writeGlobalHeader() error {
	// https://wiki.wireshark.org/Development/LibpcapFileFormat
	hdr := make([]byte, 24)
	binary.LittleEndian.PutUint32(hdr[0:4], 0xa1b2c3d4) // magic
	binary.LittleEndian.PutUint16(hdr[4:6], 2)          // major
	binary.LittleEndian.PutUint16(hdr[6:8], 4)          // minor
	binary.LittleEndian.PutUint32(hdr[8:12], 0)         // thiszone
	binary.LittleEndian.PutUint32(hdr[12:16], 0)        // sigfigs
	binary.LittleEndian.PutUint32(hdr[16:20], 262144)   // snaplen
	binary.LittleEndian.PutUint32(hdr[20:24], pcapLinkTypeEthernet)
	n, err := w.f.Write(hdr)
	w.written += int64(n)
	return err
}

// WritePacket appends one captured frame with the given timestamp.
func (w *PCAPWriter) WritePacket(ts time.Time, frame []byte) error {
	if w == nil || len(frame) == 0 {
		return nil
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.f == nil {
		return fmt.Errorf("pcap: closed")
	}
	if w.maxBytes > 0 && w.written+int64(16+len(frame)) > w.maxBytes {
		if err := w.rotateLocked(); err != nil {
			return err
		}
	}
	if ts.IsZero() {
		ts = time.Now()
	}
	sec := uint32(ts.Unix())
	usec := uint32(ts.Nanosecond() / 1000)
	incl := uint32(len(frame))
	hdr := make([]byte, 16)
	binary.LittleEndian.PutUint32(hdr[0:4], sec)
	binary.LittleEndian.PutUint32(hdr[4:8], usec)
	binary.LittleEndian.PutUint32(hdr[8:12], incl)
	binary.LittleEndian.PutUint32(hdr[12:16], incl)
	if _, err := w.f.Write(hdr); err != nil {
		return err
	}
	if _, err := w.f.Write(frame); err != nil {
		return err
	}
	w.written += int64(16 + len(frame))
	w.packets++
	return nil
}

func (w *PCAPWriter) rotateLocked() error {
	_ = w.f.Close()
	bak := w.path + ".1"
	_ = os.Remove(bak)
	if err := os.Rename(w.path, bak); err != nil {
		// recreate even if rename fails
	}
	f, err := os.OpenFile(w.path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		w.f = nil
		return err
	}
	w.f = f
	w.written = 0
	return w.writeGlobalHeader()
}

// Stats returns packets written and approximate bytes on the current file.
func (w *PCAPWriter) Stats() (packets uint64, bytes int64) {
	if w == nil {
		return 0, 0
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.packets, w.written
}

// Path returns the capture file path.
func (w *PCAPWriter) Path() string {
	if w == nil {
		return ""
	}
	return w.path
}

// Close flushes and closes the file.
func (w *PCAPWriter) Close() error {
	if w == nil {
		return nil
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.f == nil {
		return nil
	}
	err := w.f.Close()
	w.f = nil
	return err
}
