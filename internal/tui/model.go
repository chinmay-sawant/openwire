package tui

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/chinmay-sawant/openwire/internal/domain"
	"github.com/chinmay-sawant/openwire/internal/store/memory"
)

type pane int

const (
	paneHeader pane = iota
	paneGraph
	paneApps
	paneDetail
)

type chip struct {
	label string // display
	id    string // filter id: "" for all, or iface name
	x0    int
	x1    int
}

// Model is the Bubble Tea root model for OpenWire.
type Model struct {
	store  *memory.Store
	theme  Theme
	width  int
	height int

	focus       pane
	selected    int
	selectedKey string
	showDetail  bool

	apps     []domain.AppUsage
	samples  []domain.BandwidthSample
	adapters []domain.Adapter
	status   domain.Status
	flows    []domain.Flow

	filterIface   string
	toggleNames   []string // cycle order for [ and ]
	headerChips   []chip
	smoothMaxRate float64
	displayRates  map[string]float64
	smoothSamples []domain.BandwidthSample
	smoothHdrRx   float64
	smoothHdrTx   float64

	headerH int
	graphH  int
	appsH   int
	detailH int

	ready bool
}

// NewModel constructs the TUI model bound to a store.
func NewModel(store *memory.Store, themeName string) Model {
	_ = themeName
	m := Model{
		store:        store,
		theme:        DarkTheme(),
		focus:        paneApps,
		displayRates: make(map[string]float64),
		headerH:      2,
		graphH:       3,
	}
	return m
}

type tickMsg time.Time

func tickCmd() tea.Cmd {
	return tea.Tick(500*time.Millisecond, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (m Model) Init() tea.Cmd { return tickCmd() }

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true
		m.recomputeLayout()
		return m, nil

	case tickMsg:
		m.refresh()
		return m, tickCmd()

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "tab":
			m.focus = (m.focus + 1) % 4
			return m, nil
		case "shift+tab":
			m.focus = (m.focus + 3) % 4
			return m, nil
		case "[", "h", "left":
			m.cycleFilter(-1)
			return m, nil
		case "]", "l", "right":
			m.cycleFilter(1)
			return m, nil
		case "a":
			m.setFilter("")
			return m, nil
		case "up", "k":
			if m.focus == paneApps || m.focus == paneDetail || m.focus == paneHeader {
				if m.selected > 0 {
					m.selected--
					m.syncSelectedKey()
					m.loadFlows()
				}
			}
			return m, nil
		case "down", "j":
			if m.focus == paneApps || m.focus == paneDetail || m.focus == paneHeader {
				if m.selected < len(m.apps)-1 {
					m.selected++
					m.syncSelectedKey()
					m.loadFlows()
				}
			}
			return m, nil
		case "enter":
			if m.focus == paneHeader && len(m.toggleNames) > 0 {
				// enter on header cycles filter
				m.cycleFilter(1)
				return m, nil
			}
			m.showDetail = true
			m.focus = paneDetail
			m.loadFlows()
			return m, nil
		case "esc":
			m.showDetail = false
			m.focus = paneApps
			return m, nil
		case "home":
			m.selected = 0
			m.syncSelectedKey()
			m.loadFlows()
			return m, nil
		case "end":
			if len(m.apps) > 0 {
				m.selected = len(m.apps) - 1
			}
			m.syncSelectedKey()
			m.loadFlows()
			return m, nil
		}

	case tea.MouseMsg:
		// Accept press (and release on some terminals) for reliable chip clicks.
		switch msg.Action {
		case tea.MouseActionPress, tea.MouseActionRelease:
			if msg.Button == tea.MouseButtonLeft {
				m.handleClick(msg.X, msg.Y)
			}
			if msg.Button == tea.MouseButtonWheelUp {
				if m.selected > 0 {
					m.selected--
					m.syncSelectedKey()
					m.loadFlows()
				}
			}
			if msg.Button == tea.MouseButtonWheelDown {
				if m.selected < len(m.apps)-1 {
					m.selected++
					m.syncSelectedKey()
					m.loadFlows()
				}
			}
		}
		return m, nil
	}
	return m, nil
}

func (m *Model) recomputeLayout() {
	// Compact graph (no wasted blank band); give room to apps.
	m.headerH = 2
	m.graphH = 3 // 1 border-less compact block: title+chart | rates
	m.detailH = max(4, m.height/7)
	m.appsH = m.height - m.headerH - m.graphH - m.detailH - 1 // help
	if m.appsH < 6 {
		m.appsH = 6
	}
}

func (m *Model) refresh() {
	if m.store == nil {
		return
	}
	m.store.TickSample(time.Now())
	m.filterIface = m.store.IfaceFilter()
	raw := m.store.ListAppsByBandwidth(80)
	rawSamples := m.store.Samples()
	m.adapters = m.store.ListAdapters()
	m.status = m.store.Snapshot()
	m.toggleNames = buildToggleNames(m.adapters)
	// Rebuild chip hitboxes here (not only in View) so clicks see fresh geometry.
	if m.width > 0 {
		_, chips := m.renderAdapterChips(m.width)
		m.headerChips = chips
	}

	// Asymmetric EMA: moderate rise, slow fall — avoids spike→cliff look.
	maxR := 0.0
	for _, a := range raw {
		r := a.RateInBps + a.RateOutBps
		prev := m.displayRates[a.Key]
		m.displayRates[a.Key] = smoothRate(prev, r)
		if m.displayRates[a.Key] > maxR {
			maxR = m.displayRates[a.Key]
		}
	}
	m.smoothMaxRate = smoothRate(m.smoothMaxRate, maxR)
	if m.smoothMaxRate < 1 {
		m.smoothMaxRate = 1
	}
	// Smooth header totals independently.
	m.smoothHdrRx = smoothRate(m.smoothHdrRx, m.status.TotalRxBps)
	m.smoothHdrTx = smoothRate(m.smoothHdrTx, m.status.TotalTxBps)
	// Smooth graph samples (copy + EMA last point into display ring).
	m.samples = smoothSampleRing(m.smoothSamples, rawSamples)
	m.smoothSamples = m.samples
	live := make(map[string]struct{}, len(raw))
	for _, a := range raw {
		live[a.Key] = struct{}{}
	}
	for k := range m.displayRates {
		if _, ok := live[k]; !ok {
			delete(m.displayRates, k)
		}
	}

	m.apps = raw
	if m.selectedKey != "" {
		found := false
		for i, a := range m.apps {
			if a.Key == m.selectedKey {
				m.selected = i
				found = true
				break
			}
		}
		if !found {
			m.selected = max(0, len(m.apps)-1)
			m.syncSelectedKey()
		}
	} else if m.selected >= len(m.apps) {
		m.selected = max(0, len(m.apps)-1)
		m.syncSelectedKey()
	}
	if len(m.apps) == 0 {
		m.selected = 0
		m.selectedKey = ""
	}
	m.loadFlows()
}

func buildToggleNames(ads []domain.Adapter) []string {
	var names []string
	seen := map[string]struct{}{}
	for _, a := range ads {
		if a.Source == domain.AdapterSourceWindowsHost {
			id := "win:" + a.Name
			if _, ok := seen[id]; ok {
				continue
			}
			// Prefer up adapters first later — collect all
			names = append(names, id)
			seen[id] = struct{}{}
			continue
		}
		if a.Loopback {
			continue
		}
		if _, ok := seen[a.Name]; ok {
			continue
		}
		names = append(names, a.Name)
		seen[a.Name] = struct{}{}
	}
	return names
}

// smoothRate eases rates up moderately and down slowly (no cliff drops).
func smoothRate(prev, next float64) float64 {
	if prev <= 0 {
		// ease into first reading instead of snapping
		return next * 0.45
	}
	if next > prev {
		// attack
		out := 0.22*next + 0.78*prev
		// cap rise per tick (~40% of previous, min floor)
		cap := prev*1.4 + 256
		if out > cap {
			out = cap
		}
		return out
	}
	// release — long tail
	return 0.10*next + 0.90*prev
}

func smoothSampleRing(prev, raw []domain.BandwidthSample) []domain.BandwidthSample {
	if len(raw) == 0 {
		// decay previous display samples slowly toward zero
		if len(prev) == 0 {
			return nil
		}
		out := make([]domain.BandwidthSample, len(prev))
		for i, s := range prev {
			out[i] = domain.BandwidthSample{
				Time:     s.Time,
				RxBps:    s.RxBps * 0.85,
				TxBps:    s.TxBps * 0.85,
				TotalBps: s.TotalBps * 0.85,
			}
		}
		return out
	}
	out := make([]domain.BandwidthSample, len(raw))
	for i, s := range raw {
		if i < len(prev) {
			// blend each aligned point lightly; last point more responsive
			alpha := 0.25
			if i == len(raw)-1 {
				alpha = 0.30
			}
			pr := prev[i]
			// if lengths differ, match by index from end
			if len(prev) != len(raw) {
				pi := len(prev) - (len(raw) - i)
				if pi >= 0 && pi < len(prev) {
					pr = prev[pi]
				} else {
					pr = domain.BandwidthSample{}
				}
			}
			rx := alpha*s.RxBps + (1-alpha)*pr.RxBps
			tx := alpha*s.TxBps + (1-alpha)*pr.TxBps
			out[i] = domain.BandwidthSample{Time: s.Time, RxBps: rx, TxBps: tx, TotalBps: rx + tx}
		} else {
			out[i] = s
		}
	}
	// extra ease on the newest sample against previous newest
	if len(out) > 0 && len(prev) > 0 {
		last := &out[len(out)-1]
		pl := prev[len(prev)-1]
		last.RxBps = smoothRate(pl.RxBps, last.RxBps)
		last.TxBps = smoothRate(pl.TxBps, last.TxBps)
		last.TotalBps = last.RxBps + last.TxBps
	}
	return out
}

func (m *Model) cycleFilter(dir int) {
	if m.store == nil {
		return
	}
	// Order: all ("") then each capture/host adapter.
	names := make([]string, 0, len(m.toggleNames)+1)
	names = append(names, "")
	names = append(names, m.toggleNames...)
	if len(names) == 1 {
		return
	}
	cur := m.store.IfaceFilter()
	idx := 0
	for i, n := range names {
		if n == cur {
			idx = i
			break
		}
	}
	n := len(names)
	idx = (idx + dir%n + n) % n
	m.store.SetIfaceFilterExact(names[idx])
	m.filterIface = m.store.IfaceFilter()
	m.refresh()
}

func (m *Model) setFilter(id string) {
	if m.store == nil {
		return
	}
	m.store.SetIfaceFilterExact(id)
	m.filterIface = id
	m.refresh()
}

func (m *Model) syncSelectedKey() {
	if m.selected >= 0 && m.selected < len(m.apps) {
		m.selectedKey = m.apps[m.selected].Key
	}
}

func (m *Model) loadFlows() {
	if m.store == nil || m.selected < 0 || m.selected >= len(m.apps) {
		m.flows = nil
		return
	}
	m.flows = m.store.ListFlowsForApp(m.apps[m.selected].Key, 8)
}

func (m *Model) handleClick(x, y int) {
	if m.height < 10 || m.width < 10 {
		return
	}
	// Header rows 0..headerH-1: recompute chips at click time (View is value-receiver).
	if y >= 0 && y < m.headerH {
		m.focus = paneHeader
		_, chips := m.renderAdapterChips(m.width)
		m.headerChips = chips
		for _, c := range chips {
			if x >= c.x0 && x < c.x1 {
				m.setFilter(c.id)
				return
			}
		}
		// Click on empty header area → cycle next adapter.
		if y == 1 {
			m.cycleFilter(1)
		}
		return
	}
	graphEnd := m.headerH + m.graphH
	appsEnd := graphEnd + m.appsH
	switch {
	case y < graphEnd:
		m.focus = paneGraph
	case y < appsEnd:
		m.focus = paneApps
		innerTop := graphEnd + 1
		row := y - innerTop - 1
		if row >= 0 && row < len(m.apps) {
			m.selected = row
			m.syncSelectedKey()
			m.loadFlows()
		}
	default:
		m.focus = paneDetail
		m.showDetail = true
	}
}

// View implements tea.Model.
func (m Model) View() string {
	if !m.ready {
		return m.theme.Muted.Render("starting OpenWire…")
	}
	if m.width < 60 || m.height < 16 {
		return m.theme.Warn.Render(fmt.Sprintf(
			"terminal too small (%dx%d). need at least 60x16", m.width, m.height,
		))
	}
	if m.appsH == 0 {
		m.recomputeLayout()
	}

	w := m.width
	header := m.renderHeaderFull(w)
	// Compact graph: no heavy empty border padding — single thin bordered strip.
	graphInner := m.renderGraphCompact(w - 2)
	graph := m.theme.Border.Width(w - 2).Render(graphInner)
	// Ensure graph block is exactly graphH lines.
	graph = padBlock(graph, w, m.graphH)

	appsBody := m.renderAppsBody(max(1, m.appsH-2), max(1, w-4))
	detailBody := m.renderDetailBody(max(1, m.detailH-2), max(1, w-4))
	apps := padBlock(m.theme.Border.Width(w-2).Height(m.appsH-2).Render(appsBody), w, m.appsH)
	detail := padBlock(m.theme.Border.Width(w-2).Height(m.detailH-2).Render(detailBody), w, m.detailH)

	help := fitLine(m.theme.Help.Render(
		"click adapters · [ ] cycle · a=all · ↑↓ apps · tab panes · enter detail · q quit",
	), w)

	return strings.Join([]string{
		header,
		graph,
		apps,
		detail,
		padBlock(help, w, 1),
	}, "\n")
}

func (m *Model) renderHeaderFull(w int) string {
	state := "idle"
	switch m.status.Mode {
	case domain.ModeDemo:
		state = "demo"
	case domain.ModeStats:
		state = "stats"
	case domain.ModeLive:
		state = "live"
	default:
		if m.status.Running {
			state = "live"
		}
	}

	// Solid full-width header: build plain text then paint one background style.
	// (Mixing per-token backgrounds left unstyled padding = "half background".)
	rx, tx := m.smoothHdrRx, m.smoothHdrTx
	if rx == 0 && tx == 0 {
		rx, tx = m.status.TotalRxBps, m.status.TotalTxBps
	}
	row1Plain := fmt.Sprintf(" OpenWire  %s", state)
	if m.status.IsWSL2 {
		row1Plain += "  WSL2"
	}
	row1Plain += fmt.Sprintf("   ↑%-9s ↓%-9s", humanRate(tx), humanRate(rx))
	if m.status.Message != "" {
		row1Plain += "  " + truncate(m.status.Message, 28)
	}
	row1 := m.theme.HeaderBar.Width(w).MaxWidth(w).Render(truncateRunes(row1Plain, w))

	row2Body, chips := m.renderAdapterChips(w)
	m.headerChips = chips
	// Chips already include background; wrap remainder of line in HeaderBar.
	chipW := lipgloss.Width(row2Body)
	pad := ""
	if chipW < w {
		pad = strings.Repeat(" ", w-chipW)
	}
	row2 := row2Body + m.theme.HeaderBar.Render(pad)
	if lipgloss.Width(row2) > w {
		row2 = lipgloss.NewStyle().MaxWidth(w).Render(row2)
	} else if lipgloss.Width(row2) < w {
		row2 = row2 + m.theme.HeaderBar.Render(strings.Repeat(" ", w-lipgloss.Width(row2)))
	}

	return row1 + "\n" + row2
}

func (m Model) renderAdapterChips(w int) (string, []chip) {
	active := m.theme.ChipOn
	idle := m.theme.ChipOff

	var chips []chip
	var b strings.Builder
	// Start at column 0 so hitboxes match mouse X (0-based cells).
	x := 0
	b.WriteString(m.theme.HeaderBar.Render(" "))
	x++

	add := func(id, label string) {
		lab := " " + label + " "
		style := idle
		if id == m.filterIface {
			style = active
		}
		rendered := style.Render(lab)
		width := lipgloss.Width(rendered)
		chips = append(chips, chip{label: label, id: id, x0: x, x1: x + width})
		b.WriteString(rendered)
		x += width
		// gap with header background so line stays solid
		gap := m.theme.HeaderBar.Render(" ")
		b.WriteString(gap)
		x += lipgloss.Width(gap)
	}

	add("", "all")
	for _, name := range m.toggleNames {
		label := name
		if strings.HasPrefix(name, "win:") {
			label = "win:" + shortName(strings.TrimPrefix(name, "win:"), 14)
		} else {
			label = shortName(name, 12)
		}
		add(name, label)
		if x > w-10 {
			break
		}
	}
	hint := m.theme.HeaderBar.Render(" [ ]")
	b.WriteString(hint)
	return b.String(), chips
}

func truncateRunes(s string, n int) string {
	if n <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= n {
		// pad plain so Width(style) fills
		return s + strings.Repeat(" ", n-len(r))
	}
	return string(r[:n])
}

func shortName(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}

// formatAppLabel builds a list label.
// Windows apps: "win/chrome · 1234" (name + pid).
// Linux/local apps: process/binary name only (no trailing pid clutter).
func formatAppLabel(name string, pid int) string {
	name = strings.TrimSpace(name)
	if name == "" || name == "unknown" {
		if pid > 0 {
			return fmt.Sprintf("pid:%d", pid)
		}
		return "unknown"
	}
	if strings.HasPrefix(name, "pid:") {
		return name
	}
	// Windows host processes — show name + pid as requested.
	if strings.HasPrefix(name, "win/") {
		base := strings.TrimPrefix(name, "win/")
		if strings.HasPrefix(base, "pid:") {
			// unresolved name — still show pid once
			if pid > 0 {
				return fmt.Sprintf("win/? · %d", pid)
			}
			return name
		}
		if pid > 0 && !strings.Contains(name, "·") {
			return fmt.Sprintf("%s · %d", name, pid)
		}
		return name
	}
	// Linux / demo / other: name only (binary/comm), not "node · 12345".
	return name
}

func (m Model) renderGraphCompact(innerW int) string {
	focus := " "
	if m.focus == paneGraph {
		focus = "▶"
	}
	filter := "all"
	if m.filterIface != "" {
		filter = m.filterIface
	}
	title := m.theme.Accent.Render(fmt.Sprintf("%s Bandwidth · %s", focus, truncate(filter, 24)))
	if len(m.samples) == 0 {
		return title + "\n" + m.theme.Muted.Render(" waiting for traffic…")
	}
	chartW := max(12, innerW-2)
	chart := m.theme.Graph.Render(sparkline(m.samples, chartW, 1))
	last := m.samples[len(m.samples)-1]
	// Put rates on same visual band as chart (2 lines total inside border).
	stats := m.theme.Rx.Render(fmt.Sprintf("↓%-9s", humanRate(last.RxBps))) + " " +
		m.theme.Tx.Render(fmt.Sprintf("↑%-9s", humanRate(last.TxBps))) + " " +
		m.theme.Muted.Render(fmt.Sprintf("Σ%-9s", humanRate(last.TotalBps)))
	// Single content line: chart; second: title+stats squeezed
	return fitLine(title+"  "+stats, innerW) + "\n" + fitLine(chart, innerW)
}

func (m Model) renderAppsBody(rows, cols int) string {
	focusMark := " "
	if m.focus == paneApps {
		focusMark = "▶"
	}
	title := m.theme.Accent.Render(focusMark + " Applications (by usage)")
	lines := []string{fitLine(title, cols)}
	if len(m.apps) == 0 {
		lines = append(lines, fitLine(m.theme.Muted.Render(" no apps yet"), cols))
		return joinFixed(lines, rows, cols)
	}
	barW := max(8, min(20, cols/5))
	nameW := 18
	maxScale := m.smoothMaxRate
	if maxScale < 1 {
		maxScale = 1
	}
	dataRows := rows - 1
	if dataRows < 1 {
		dataRows = 1
	}
	for i, a := range m.apps {
		if i >= dataRows {
			break
		}
		rate := m.displayRates[a.Key]
		if rate <= 0 {
			rate = a.RateInBps + a.RateOutBps
		}
		bar := rateBar(rate/maxScale, barW)
		name := formatAppLabel(a.Name, a.PID)
		line := fmt.Sprintf("%s %s %10s  ↑%-8s ↓%-8s",
			padRight(truncate(name, nameW), nameW),
			bar,
			humanRate(rate),
			humanBytes(a.BytesOut),
			humanBytes(a.BytesIn),
		)
		if i == m.selected {
			line = m.theme.Selected.Render("▶" + line)
		} else {
			line = m.theme.App.Render(" " + line)
		}
		lines = append(lines, fitLine(line, cols))
	}
	return joinFixed(lines, rows, cols)
}

func (m Model) renderDetailBody(rows, cols int) string {
	focusMark := " "
	if m.focus == paneDetail {
		focusMark = "▶"
	}
	title := m.theme.Accent.Render(focusMark + " Detail")
	lines := []string{fitLine(title, cols)}
	if m.selected < 0 || m.selected >= len(m.apps) {
		lines = append(lines, fitLine(m.theme.Muted.Render(" select an application"), cols))
		return joinFixed(lines, rows, cols)
	}
	a := m.apps[m.selected]
	head := fmt.Sprintf("%s · flows %d", truncate(formatAppLabel(a.Name, a.PID), 28), a.Flows)
	if a.Path != "" {
		head += " · " + truncate(a.Path, max(8, cols/3))
	}
	lines = append(lines, fitLine(m.theme.App.Render(head), cols))
	if len(m.flows) == 0 {
		lines = append(lines, fitLine(m.theme.Muted.Render(" no flows"), cols))
		return joinFixed(lines, rows, cols)
	}
	for _, f := range m.flows {
		if len(lines) >= rows {
			break
		}
		line := fmt.Sprintf("%s %s %s:%d ↔ %s:%d  in %s out %s",
			f.Key.Iface, f.Key.Protocol, f.Key.SrcIP, f.Key.SrcPort, f.Key.DstIP, f.Key.DstPort,
			humanBytes(f.BytesIn), humanBytes(f.BytesOut),
		)
		lines = append(lines, fitLine(m.theme.Muted.Render(truncate(line, cols)), cols))
	}
	return joinFixed(lines, rows, cols)
}

func joinFixed(lines []string, rows, cols int) string {
	for len(lines) < rows {
		lines = append(lines, strings.Repeat(" ", cols))
	}
	if len(lines) > rows {
		lines = lines[:rows]
	}
	for i := range lines {
		lines[i] = fitLine(lines[i], cols)
	}
	return strings.Join(lines, "\n")
}

func padBlock(s string, cols, rows int) string {
	parts := strings.Split(s, "\n")
	for i := range parts {
		parts[i] = fitLine(parts[i], cols)
	}
	for len(parts) < rows {
		parts = append(parts, strings.Repeat(" ", cols))
	}
	if len(parts) > rows {
		parts = parts[:rows]
	}
	return strings.Join(parts, "\n")
}

func fitLine(s string, width int) string {
	if width <= 0 {
		return ""
	}
	plain := lipgloss.Width(s)
	if plain == width {
		return s
	}
	if plain > width {
		return lipgloss.NewStyle().MaxWidth(width).Render(s)
	}
	return s + strings.Repeat(" ", width-plain)
}

func padRight(s string, n int) string {
	w := utf8.RuneCountInString(s)
	if w >= n {
		return s
	}
	return s + strings.Repeat(" ", n-w)
}

func sparkline(samples []domain.BandwidthSample, width, height int) string {
	if width < 4 {
		width = 4
	}
	start := 0
	if len(samples) > width {
		start = len(samples) - width
	}
	slice := samples[start:]
	maxV := 0.0
	for _, s := range slice {
		if s.TotalBps > maxV {
			maxV = s.TotalBps
		}
	}
	if maxV <= 0 {
		maxV = 1
	}
	blocks := []rune("▁▂▃▄▅▆▇█")
	var row strings.Builder
	for _, s := range slice {
		idx := int((s.TotalBps / maxV) * float64(len(blocks)-1))
		if idx < 0 {
			idx = 0
		}
		if idx >= len(blocks) {
			idx = len(blocks) - 1
		}
		row.WriteRune(blocks[idx])
	}
	for utf8.RuneCountInString(row.String()) < width {
		row.WriteRune('▁')
	}
	_ = height
	return row.String()
}

func rateBar(frac float64, width int) string {
	if frac < 0 {
		frac = 0
	}
	if frac > 1 {
		frac = 1
	}
	filled := int(frac * float64(width))
	if filled > width {
		filled = width
	}
	return strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
}

func humanRate(bps float64) string {
	return humanBytes(uint64(bps)) + "/s"
}

func humanBytes(n uint64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%4d B", n)
	}
	div, exp := uint64(unit), 0
	for v := n / unit; v >= unit; v /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%4.1f%cB", float64(n)/float64(div), "KMGTPE"[exp])
}

func truncate(s string, n int) string {
	if n <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	if n <= 1 {
		return string(r[:n])
	}
	return string(r[:n-1]) + "…"
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
