package capture

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestPCAPWriterRoundTripHeader(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.pcap")
	w, err := NewPCAPWriter(path, 0)
	if err != nil {
		t.Fatal(err)
	}
	frame := make([]byte, 64)
	// fake eth header
	binary.BigEndian.PutUint16(frame[12:14], 0x0800)
	if err := w.WritePacket(time.Unix(1_700_000_000, 123_000_000), frame); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) < 24+16+64 {
		t.Fatalf("file too small: %d", len(data))
	}
	magic := binary.LittleEndian.Uint32(data[0:4])
	if magic != 0xa1b2c3d4 {
		t.Fatalf("magic %x", magic)
	}
	link := binary.LittleEndian.Uint32(data[20:24])
	if link != pcapLinkTypeEthernet {
		t.Fatalf("link %d", link)
	}
	incl := binary.LittleEndian.Uint32(data[24+8 : 24+12])
	if incl != 64 {
		t.Fatalf("incl len %d", incl)
	}
	pkts, _ := w.Stats()
	if pkts != 0 { // closed — stats still hold
		// Stats after close still returns counters
	}
	// re-open writer stats were on closed w
	w2, _ := NewPCAPWriter(filepath.Join(dir, "t2.pcap"), 100)
	_ = w2.WritePacket(time.Now(), frame)
	p, b := w2.Stats()
	if p != 1 || b < 24+16 {
		t.Fatalf("stats p=%d b=%d", p, b)
	}
	_ = w2.Close()
}

func TestPCAPWriterRotation(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "rot.pcap")
	// tiny max so second packet rotates
	w, err := NewPCAPWriter(path, 24+16+32)
	if err != nil {
		t.Fatal(err)
	}
	frame := make([]byte, 40)
	_ = w.WritePacket(time.Now(), frame)
	_ = w.WritePacket(time.Now(), frame)
	_ = w.Close()
	if _, err := os.Stat(path + ".1"); err != nil {
		t.Fatalf("expected rotated backup: %v", err)
	}
}
