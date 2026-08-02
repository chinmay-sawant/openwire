// Package domain holds pure read models for OpenWire. No TUI or capture imports.
package domain

import "time"

// AdapterSource labels where an adapter inventory entry came from.
type AdapterSource string

const (
	AdapterSourceLinux       AdapterSource = "linux"
	AdapterSourceWindowsHost AdapterSource = "windows-host"
	AdapterSourceDemo        AdapterSource = "demo"
)

// Adapter is a network interface OpenWire knows about.
type Adapter struct {
	Name       string
	Index      int
	Hardware   string
	IPv4       []string
	IPv6       []string
	Up         bool
	Loopback   bool
	Source     AdapterSource
	RxBytes    uint64
	TxBytes    uint64
	RxRateBps  float64
	TxRateBps  float64
}

// Direction is best-effort traffic direction relative to the host.
type Direction string

const (
	DirectionRx      Direction = "rx"
	DirectionTx      Direction = "tx"
	DirectionUnknown Direction = "unknown"
)

// Protocol is the L4 (or raw) protocol of a flow.
type Protocol string

const (
	ProtoTCP  Protocol = "tcp"
	ProtoUDP  Protocol = "udp"
	ProtoICMP Protocol = "icmp"
	ProtoOther Protocol = "other"
)

// Observation is one captured packet or synthetic byte event from a capture engine.
type Observation struct {
	Time       time.Time
	Iface      string
	Direction  Direction
	Length     int
	Protocol   Protocol
	SrcIP      string
	DstIP      string
	SrcPort    uint16
	DstPort    uint16
	// AppHint is optional process name from a demo engine or attribution layer.
	AppHint string
	PIDHint int
}

// FlowKey identifies a bidirectional-normalized conversation when possible.
type FlowKey struct {
	Iface    string
	Protocol Protocol
	SrcIP    string
	DstIP    string
	SrcPort  uint16
	DstPort  uint16
}

// Flow is an aggregated conversation in the store.
type Flow struct {
	Key       FlowKey
	BytesIn   uint64
	BytesOut  uint64
	Packets   uint64
	FirstSeen time.Time
	LastSeen  time.Time
	PID       int
	AppName   string
}

// AppUsage is per-application bandwidth ranking material for the default UI.
type AppUsage struct {
	Key       string // stable key: name or "pid:N" or "unknown"
	Name      string
	PID       int
	Path      string
	BytesIn   uint64
	BytesOut  uint64
	RateInBps float64
	RateOutBps float64
	Flows     int
}

// BandwidthSample is one point on the GlassWire-style graph.
type BandwidthSample struct {
	Time      time.Time
	RxBps     float64
	TxBps     float64
	TotalBps  float64
}

// CaptureMode describes how the runtime is obtaining data.
type CaptureMode string

const (
	ModeLive  CaptureMode = "live"  // AF_PACKET packet capture (privileged)
	ModeStats CaptureMode = "stats" // unprivileged /proc interface + socket sampling
	ModeDemo  CaptureMode = "demo"  // synthetic traffic
)

// Status is a snapshot of runtime health for the status bar.
type Status struct {
	Mode          CaptureMode
	Running       bool
	PrivilegeOK   bool
	Message       string
	Adapters      []string
	Dropped       uint64
	TotalRxBytes  uint64
	TotalTxBytes  uint64
	TotalRxBps    float64
	TotalTxBps    float64
	AppCount      int
	FlowCount     int
	IsWSL2        bool
}
