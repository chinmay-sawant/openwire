// Package memory is a bounded in-memory traffic store.
package memory

import (
	"sort"
	"sync"
	"time"

	"github.com/chinmay-sawant/openwire/internal/domain"
)

// Config bounds store growth.
type Config struct {
	MaxFlows   int
	MaxSamples int
	MaxApps    int
	// RateWindow is used for sliding rate estimates.
	RateWindow time.Duration
}

func (c Config) withDefaults() Config {
	if c.MaxFlows <= 0 {
		c.MaxFlows = 4096
	}
	if c.MaxSamples <= 0 {
		c.MaxSamples = 120
	}
	if c.MaxApps <= 0 {
		c.MaxApps = 512
	}
	if c.RateWindow <= 0 {
		c.RateWindow = 3 * time.Second
	}
	return c
}

type appAgg struct {
	name       string
	pid        int
	path       string
	bytesIn    uint64
	bytesOut   uint64
	flows      map[string]struct{}
	// rate fields updated on sample ticks
	rateInBps  float64
	rateOutBps float64
	prevIn     uint64
	prevOut    uint64
}

type flowRec struct {
	flow  domain.Flow
	touch time.Time
}

// Store is a thread-safe in-memory traffic store.
type Store struct {
	mu       sync.RWMutex
	cfg      Config
	flows    map[string]*flowRec
	apps     map[string]*appAgg
	samples  []domain.BandwidthSample
	adapters []domain.Adapter
	status   domain.Status

	// sampling for graph
	rxSinceSample uint64
	txSinceSample uint64
	lastSampleAt  time.Time
}

// New creates a Store with the given bounds.
func New(cfg Config) *Store {
	cfg = cfg.withDefaults()
	return &Store{
		cfg:    cfg,
		flows:  make(map[string]*flowRec),
		apps:   make(map[string]*appAgg),
		status: domain.Status{Running: false},
	}
}

// SetStatus replaces the runtime status fields (mode, privilege, message, …).
func (s *Store) SetStatus(st domain.Status) {
	s.mu.Lock()
	defer s.mu.Unlock()
	// Preserve live totals computed from ingest.
	st.TotalRxBytes = s.status.TotalRxBytes
	st.TotalTxBytes = s.status.TotalTxBytes
	st.TotalRxBps = s.status.TotalRxBps
	st.TotalTxBps = s.status.TotalTxBps
	st.AppCount = len(s.apps)
	st.FlowCount = len(s.flows)
	s.status = st
}

// SetAdapters replaces the adapter inventory shown in the UI.
func (s *Store) SetAdapters(adapters []domain.Adapter) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.adapters = append([]domain.Adapter(nil), adapters...)
	names := make([]string, 0, len(adapters))
	for _, a := range adapters {
		names = append(names, a.Name)
	}
	s.status.Adapters = names
}

// Ingest applies one observation.
func (s *Store) Ingest(o domain.Observation) {
	if o.Time.IsZero() {
		o.Time = time.Now()
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	key := flowKeyString(o)
	rec, ok := s.flows[key]
	if !ok {
		if len(s.flows) >= s.cfg.MaxFlows {
			s.evictOldestFlow()
		}
		rec = &flowRec{
			flow: domain.Flow{
				Key: domain.FlowKey{
					Iface:    o.Iface,
					Protocol: o.Protocol,
					SrcIP:    o.SrcIP,
					DstIP:    o.DstIP,
					SrcPort:  o.SrcPort,
					DstPort:  o.DstPort,
				},
				FirstSeen: o.Time,
			},
		}
		s.flows[key] = rec
	}
	rec.touch = o.Time
	rec.flow.LastSeen = o.Time
	rec.flow.Packets++

	in, out := splitDirection(o)
	rec.flow.BytesIn += in
	rec.flow.BytesOut += out

	s.status.TotalRxBytes += in
	s.status.TotalTxBytes += out
	s.rxSinceSample += in
	s.txSinceSample += out

	appKey, appName, pid := appIdentity(o, rec)
	if appName != "" {
		rec.flow.AppName = appName
	}
	if pid > 0 {
		rec.flow.PID = pid
	}
	agg, ok := s.apps[appKey]
	if !ok {
		if len(s.apps) >= s.cfg.MaxApps {
			s.evictColdestApp()
		}
		agg = &appAgg{
			name:  appName,
			pid:   pid,
			flows: make(map[string]struct{}),
		}
		s.apps[appKey] = agg
	}
	agg.bytesIn += in
	agg.bytesOut += out
	agg.flows[key] = struct{}{}
	if agg.name == "" || agg.name == "unknown" {
		agg.name = appName
	}
	if pid > 0 {
		agg.pid = pid
	}

	s.maybeSampleLocked(o.Time)
	s.status.AppCount = len(s.apps)
	s.status.FlowCount = len(s.flows)
}

// TickSample forces a graph sample using elapsed wall time (for quiet periods).
func (s *Store) TickSample(now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.maybeSampleLocked(now)
}

func (s *Store) maybeSampleLocked(now time.Time) {
	if s.lastSampleAt.IsZero() {
		s.lastSampleAt = now
		return
	}
	elapsed := now.Sub(s.lastSampleAt)
	if elapsed < time.Second {
		return
	}
	secs := elapsed.Seconds()
	if secs <= 0 {
		return
	}
	rx := float64(s.rxSinceSample) / secs
	tx := float64(s.txSinceSample) / secs
	s.samples = append(s.samples, domain.BandwidthSample{
		Time:     now,
		RxBps:    rx,
		TxBps:    tx,
		TotalBps: rx + tx,
	})
	if len(s.samples) > s.cfg.MaxSamples {
		s.samples = append([]domain.BandwidthSample(nil), s.samples[len(s.samples)-s.cfg.MaxSamples:]...)
	}
	s.status.TotalRxBps = rx
	s.status.TotalTxBps = tx
	s.rxSinceSample = 0
	s.txSinceSample = 0
	s.lastSampleAt = now

	// Per-app rates from byte deltas since last sample.
	for _, agg := range s.apps {
		din := agg.bytesIn - agg.prevIn
		dout := agg.bytesOut - agg.prevOut
		agg.rateInBps = float64(din) / secs
		agg.rateOutBps = float64(dout) / secs
		agg.prevIn = agg.bytesIn
		agg.prevOut = agg.bytesOut
	}
}

// ListAppsByBandwidth returns top apps by current rate then total bytes.
func (s *Store) ListAppsByBandwidth(limit int) []domain.AppUsage {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]domain.AppUsage, 0, len(s.apps))
	for k, agg := range s.apps {
		out = append(out, domain.AppUsage{
			Key:        k,
			Name:       agg.name,
			PID:        agg.pid,
			Path:       agg.path,
			BytesIn:    agg.bytesIn,
			BytesOut:   agg.bytesOut,
			RateInBps:  agg.rateInBps,
			RateOutBps: agg.rateOutBps,
			Flows:      len(agg.flows),
		})
	}
	sort.Slice(out, func(i, j int) bool {
		ri := out[i].RateInBps + out[i].RateOutBps
		rj := out[j].RateInBps + out[j].RateOutBps
		if ri != rj {
			return ri > rj
		}
		ti := out[i].BytesIn + out[i].BytesOut
		tj := out[j].BytesIn + out[j].BytesOut
		return ti > tj
	})
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}

// Samples returns a copy of the bandwidth ring.
func (s *Store) Samples() []domain.BandwidthSample {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]domain.BandwidthSample(nil), s.samples...)
}

// ListAdapters returns the last inventory.
func (s *Store) ListAdapters() []domain.Adapter {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]domain.Adapter(nil), s.adapters...)
}

// ListFlowsForApp returns recent flows for an app key.
func (s *Store) ListFlowsForApp(appKey string, limit int) []domain.Flow {
	s.mu.RLock()
	defer s.mu.RUnlock()
	agg, ok := s.apps[appKey]
	if !ok {
		return nil
	}
	flows := make([]domain.Flow, 0, len(agg.flows))
	for fk := range agg.flows {
		if rec, ok := s.flows[fk]; ok {
			flows = append(flows, rec.flow)
		}
	}
	sort.Slice(flows, func(i, j int) bool {
		return flows[i].BytesIn+flows[i].BytesOut > flows[j].BytesIn+flows[j].BytesOut
	})
	if limit > 0 && len(flows) > limit {
		flows = flows[:limit]
	}
	return flows
}

// Snapshot returns current status.
func (s *Store) Snapshot() domain.Status {
	s.mu.RLock()
	defer s.mu.RUnlock()
	st := s.status
	st.AppCount = len(s.apps)
	st.FlowCount = len(s.flows)
	st.Adapters = append([]string(nil), s.status.Adapters...)
	return st
}

// SetAppPath sets the executable path for an app key when attribution learns it.
func (s *Store) SetAppPath(appKey, path string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if agg, ok := s.apps[appKey]; ok {
		agg.path = path
	}
}

func (s *Store) evictOldestFlow() {
	var oldestKey string
	var oldest time.Time
	first := true
	for k, rec := range s.flows {
		if first || rec.touch.Before(oldest) {
			oldest = rec.touch
			oldestKey = k
			first = false
		}
	}
	if oldestKey == "" {
		return
	}
	delete(s.flows, oldestKey)
	for _, agg := range s.apps {
		delete(agg.flows, oldestKey)
	}
}

func (s *Store) evictColdestApp() {
	var coldKey string
	var coldBytes uint64
	first := true
	for k, agg := range s.apps {
		b := agg.bytesIn + agg.bytesOut
		if first || b < coldBytes {
			coldBytes = b
			coldKey = k
			first = false
		}
	}
	if coldKey != "" {
		delete(s.apps, coldKey)
	}
}

func flowKeyString(o domain.Observation) string {
	// Normalize endpoints so A→B and B→A share a key when possible.
	a := o.SrcIP
	ap := o.SrcPort
	b := o.DstIP
	bp := o.DstPort
	if a > b || (a == b && ap > bp) {
		a, ap, b, bp = b, bp, a, ap
	}
	return string(o.Protocol) + "|" + o.Iface + "|" + a + ":" + u16(ap) + "-" + b + ":" + u16(bp)
}

func u16(v uint16) string {
	// small alloc-friendly decimal
	if v == 0 {
		return "0"
	}
	var buf [5]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	return string(buf[i:])
}

func splitDirection(o domain.Observation) (in, out uint64) {
	n := uint64(o.Length)
	switch o.Direction {
	case domain.DirectionRx:
		return n, 0
	case domain.DirectionTx:
		return 0, n
	default:
		// Unknown: count as rx for totals so graph still moves.
		return n, 0
	}
}

func appIdentity(o domain.Observation, rec *flowRec) (key, name string, pid int) {
	if o.AppHint != "" {
		name = o.AppHint
	} else if rec.flow.AppName != "" {
		name = rec.flow.AppName
	} else {
		name = "unknown"
	}
	if o.PIDHint > 0 {
		pid = o.PIDHint
	} else {
		pid = rec.flow.PID
	}
	if name != "unknown" {
		key = name
		if pid > 0 {
			key = name + "#" + itoa(pid)
		}
		return key, name, pid
	}
	if pid > 0 {
		return "pid:" + itoa(pid), "pid:" + itoa(pid), pid
	}
	return "unknown", "unknown", 0
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [12]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
