// Package memory is a bounded in-memory traffic store.
package memory

import (
	"sort"
	"strings"
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

	// filterIface filters queries: "" = all adapters.
	filterIface string

	// sampling for graph (all + per-iface)
	rxSinceSample uint64
	txSinceSample uint64
	lastSampleAt  time.Time
	rxByIface     map[string]uint64
	txByIface     map[string]uint64
	samplesByIface map[string][]domain.BandwidthSample
}

// New creates a Store with the given bounds.
func New(cfg Config) *Store {
	cfg = cfg.withDefaults()
	return &Store{
		cfg:            cfg,
		flows:          make(map[string]*flowRec),
		apps:           make(map[string]*appAgg),
		status:         domain.Status{Running: false},
		rxByIface:      make(map[string]uint64),
		txByIface:      make(map[string]uint64),
		samplesByIface: make(map[string][]domain.BandwidthSample),
	}
}

// SetIfaceFilter sets the active adapter filter ("" = all).
func (s *Store) SetIfaceFilter(iface string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.filterIface = iface
}

// IfaceFilter returns the current adapter filter ("" = all).
func (s *Store) IfaceFilter() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.filterIface
}

// CycleIfaceFilter cycles: all → first capture iface → … → all.
// names should be the toggle list shown in the UI.
func (s *Store) CycleIfaceFilter(names []string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(names) == 0 {
		s.filterIface = ""
		return ""
	}
	if s.filterIface == "" {
		s.filterIface = names[0]
		return s.filterIface
	}
	for i, n := range names {
		if n == s.filterIface {
			if i+1 < len(names) {
				s.filterIface = names[i+1]
			} else {
				s.filterIface = ""
			}
			return s.filterIface
		}
	}
	s.filterIface = ""
	return ""
}

// SetIfaceFilterExact sets filter if name is known or empty for all.
func (s *Store) SetIfaceFilterExact(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.filterIface = name
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
	if o.Iface != "" {
		s.rxByIface[o.Iface] += in
		s.txByIface[o.Iface] += out
	}

	appKey, appName, pid := appIdentity(o, rec)
	if appName != "" && appName != "unknown" {
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
	if betterName(appName, agg.name) {
		agg.name = appName
	}
	if pid > 0 {
		agg.pid = pid
	}

	s.maybeSampleLocked(o.Time)
	s.status.AppCount = len(s.apps)
	s.status.FlowCount = len(s.flows)
}

func betterName(newName, oldName string) bool {
	if newName == "" || newName == "unknown" {
		return false
	}
	if oldName == "" || oldName == "unknown" || strings.HasPrefix(oldName, "pid:") {
		return true
	}
	// Prefer win/ process names over aggregate windows-host.
	if oldName == "windows-host" && newName != "windows-host" {
		return true
	}
	return false
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
	// Per-iface samples for adapter toggle graph.
	for iface, rxi := range s.rxByIface {
		txi := s.txByIface[iface]
		rRate := float64(rxi) / secs
		tRate := float64(txi) / secs
		ring := append(s.samplesByIface[iface], domain.BandwidthSample{
			Time: now, RxBps: rRate, TxBps: tRate, TotalBps: rRate + tRate,
		})
		if len(ring) > s.cfg.MaxSamples {
			ring = append([]domain.BandwidthSample(nil), ring[len(ring)-s.cfg.MaxSamples:]...)
		}
		s.samplesByIface[iface] = ring
	}
	s.rxSinceSample = 0
	s.txSinceSample = 0
	s.rxByIface = make(map[string]uint64)
	s.txByIface = make(map[string]uint64)
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
// When an iface filter is set, apps are rebuilt from flows on that iface only.
func (s *Store) ListAppsByBandwidth(limit int) []domain.AppUsage {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.filterIface != "" {
		return s.listAppsFilteredLocked(limit, s.filterIface)
	}
	out := make([]domain.AppUsage, 0, len(s.apps))
	for k, agg := range s.apps {
		out = append(out, domain.AppUsage{
			Key:        k,
			Name:       displayName(agg.name, agg.pid),
			PID:        agg.pid,
			Path:       agg.path,
			BytesIn:    agg.bytesIn,
			BytesOut:   agg.bytesOut,
			RateInBps:  agg.rateInBps,
			RateOutBps: agg.rateOutBps,
			Flows:      len(agg.flows),
		})
	}
	return sortLimitApps(out, limit)
}

func (s *Store) listAppsFilteredLocked(limit int, iface string) []domain.AppUsage {
	type acc struct {
		name     string
		pid      int
		path     string
		bytesIn  uint64
		bytesOut uint64
		flows    int
	}
	byKey := map[string]*acc{}
	for _, rec := range s.flows {
		if !ifaceMatch(rec.flow.Key.Iface, iface) {
			continue
		}
		name := rec.flow.AppName
		pid := rec.flow.PID
		if name == "" {
			name = "unknown"
		}
		key := name
		if pid > 0 {
			key = name + "#" + itoa(pid)
		}
		a, ok := byKey[key]
		if !ok {
			a = &acc{name: name, pid: pid}
			// path from global app map if present
			if g, ok := s.apps[key]; ok {
				a.path = g.path
				if betterName(g.name, a.name) {
					a.name = g.name
				}
			}
			byKey[key] = a
		}
		a.bytesIn += rec.flow.BytesIn
		a.bytesOut += rec.flow.BytesOut
		a.flows++
	}
	out := make([]domain.AppUsage, 0, len(byKey))
	for k, a := range byKey {
		// Use rates from global agg when available (same key).
		var rin, rout float64
		if g, ok := s.apps[k]; ok {
			// Scale global rate by share of bytes on this iface if possible.
			total := g.bytesIn + g.bytesOut
			local := a.bytesIn + a.bytesOut
			if total > 0 {
				share := float64(local) / float64(total)
				rin = g.rateInBps * share
				rout = g.rateOutBps * share
			}
		}
		out = append(out, domain.AppUsage{
			Key: k, Name: displayName(a.name, a.pid), PID: a.pid, Path: a.path,
			BytesIn: a.bytesIn, BytesOut: a.bytesOut,
			RateInBps: rin, RateOutBps: rout, Flows: a.flows,
		})
	}
	return sortLimitApps(out, limit)
}

func ifaceMatch(flowIface, filter string) bool {
	if filter == "" {
		return true
	}
	if flowIface == filter {
		return true
	}
	// win:Wi-Fi filter matches observations tagged win:Wi-Fi
	return false
}

func displayName(name string, pid int) string {
	if name == "" || name == "unknown" {
		if pid > 0 {
			return "pid:" + itoa(pid)
		}
		return "unknown"
	}
	return name
}

func sortLimitApps(out []domain.AppUsage, limit int) []domain.AppUsage {
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

// Samples returns the bandwidth ring for the active filter (or all).
func (s *Store) Samples() []domain.BandwidthSample {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.filterIface != "" {
		if ring, ok := s.samplesByIface[s.filterIface]; ok {
			return append([]domain.BandwidthSample(nil), ring...)
		}
		return nil
	}
	return append([]domain.BandwidthSample(nil), s.samples...)
}

// ListAdapters returns the last inventory.
func (s *Store) ListAdapters() []domain.Adapter {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]domain.Adapter(nil), s.adapters...)
}

// ListFlowsForApp returns recent flows for an app key (honors iface filter).
func (s *Store) ListFlowsForApp(appKey string, limit int) []domain.Flow {
	s.mu.RLock()
	defer s.mu.RUnlock()
	agg, ok := s.apps[appKey]
	if !ok {
		// Filtered rebuild may not have global app entry; scan flows.
		return s.listFlowsByAppKeyLocked(appKey, limit)
	}
	flows := make([]domain.Flow, 0, len(agg.flows))
	for fk := range agg.flows {
		if rec, ok := s.flows[fk]; ok {
			if s.filterIface != "" && !ifaceMatch(rec.flow.Key.Iface, s.filterIface) {
				continue
			}
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

func (s *Store) listFlowsByAppKeyLocked(appKey string, limit int) []domain.Flow {
	var flows []domain.Flow
	for _, rec := range s.flows {
		if s.filterIface != "" && !ifaceMatch(rec.flow.Key.Iface, s.filterIface) {
			continue
		}
		name := rec.flow.AppName
		pid := rec.flow.PID
		key := name
		if pid > 0 {
			key = name + "#" + itoa(pid)
		}
		if key != appKey && name != appKey {
			continue
		}
		flows = append(flows, rec.flow)
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

// LoadSamples prepends historical samples (e.g. from SQLite) into the ring.
// Existing live samples are kept after history; the ring is then trimmed to MaxSamples.
func (s *Store) LoadSamples(history []domain.BandwidthSample) {
	if len(history) == 0 {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	merged := make([]domain.BandwidthSample, 0, len(history)+len(s.samples))
	merged = append(merged, history...)
	merged = append(merged, s.samples...)
	if len(merged) > s.cfg.MaxSamples {
		merged = merged[len(merged)-s.cfg.MaxSamples:]
	}
	s.samples = merged
	if len(s.samples) > 0 {
		last := s.samples[len(s.samples)-1]
		s.lastSampleAt = last.Time
		s.status.TotalRxBps = last.RxBps
		s.status.TotalTxBps = last.TxBps
	}
}

// SamplesSince returns samples with Time strictly after t (for incremental flush).
func (s *Store) SamplesSince(t time.Time) []domain.BandwidthSample {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if t.IsZero() {
		return append([]domain.BandwidthSample(nil), s.samples...)
	}
	out := make([]domain.BandwidthSample, 0, len(s.samples))
	for _, sm := range s.samples {
		if sm.Time.After(t) {
			out = append(out, sm)
		}
	}
	return out
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
	if o.PIDHint > 0 {
		pid = o.PIDHint
	} else {
		pid = rec.flow.PID
	}
	switch {
	case o.AppHint != "" && o.AppHint != "unknown":
		name = o.AppHint
	case rec.flow.AppName != "" && rec.flow.AppName != "unknown":
		name = rec.flow.AppName
	case pid > 0:
		name = "pid:" + itoa(pid)
	default:
		name = "unknown"
	}
	// Never key aggregate as bare unknown when we have a pid.
	if name == "unknown" && pid > 0 {
		name = "pid:" + itoa(pid)
	}
	key = name
	if pid > 0 && !strings.Contains(name, "#") {
		key = name + "#" + itoa(pid)
	}
	return key, name, pid
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
