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

// pane identifiers for tab navigation.
type pane int

const (
	paneGraph pane = iota
	paneApps
	paneDetail
)

// Model is the Bubble Tea root model for OpenWire.
type Model struct {
	store  *memory.Store
	theme  Theme
	width  int
	height int

	focus        pane
	selected     int
	selectedKey  string // stable selection across re-sorts
	showDetail   bool

	apps     []domain.AppUsage
	samples  []domain.BandwidthSample
	adapters []domain.Adapter
	status   domain.Status
	flows    []domain.Flow

	// display smoothing — reduces bar/rate thrash
	smoothMaxRate float64
	displayRates  map[string]float64 // app key -> EMA of rate

	// fixed layout slots (set on resize / each view from height)
	graphH  int
	appsH   int
	detailH int

	// last frame cache for flicker reduction when nothing meaningful changed
	lastView   string
	frameSeq   uint64
	ready      bool
}

// NewModel constructs the TUI model bound to a store.
func NewModel(store *memory.Store, themeName string) Model {
	_ = themeName
	return Model{
		store:        store,
		theme:        DarkTheme(),
		focus:        paneApps,
		displayRates: make(map[string]float64),
	}
}

type tickMsg time.Time

func tickCmd() tea.Cmd {
	// ~2 Hz is enough for bandwidth UI and reduces full-frame redraws.
	return tea.Tick(500*time.Millisecond, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd {
	return tickCmd()
}

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true
		m.recomputeLayout()
		m.lastView = "" // force redraw
		return m, nil

	case tickMsg:
		m.refresh()
		return m, tickCmd()

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "tab":
			m.focus = (m.focus + 1) % 3
			m.lastView = ""
			return m, nil
		case "shift+tab":
			m.focus = (m.focus + 2) % 3
			m.lastView = ""
			return m, nil
		case "up", "k":
			if m.focus == paneApps || m.focus == paneDetail {
				if m.selected > 0 {
					m.selected--
					m.syncSelectedKey()
					m.loadFlows()
					m.lastView = ""
				}
			}
			return m, nil
		case "down", "j":
			if m.focus == paneApps || m.focus == paneDetail {
				if m.selected < len(m.apps)-1 {
					m.selected++
					m.syncSelectedKey()
					m.loadFlows()
					m.lastView = ""
				}
			}
			return m, nil
		case "enter":
			m.showDetail = true
			m.focus = paneDetail
			m.loadFlows()
			m.lastView = ""
			return m, nil
		case "esc":
			m.showDetail = false
			m.focus = paneApps
			m.lastView = ""
			return m, nil
		case "home":
			m.selected = 0
			m.syncSelectedKey()
			m.loadFlows()
			m.lastView = ""
			return m, nil
		case "end":
			if len(m.apps) > 0 {
				m.selected = len(m.apps) - 1
			}
			m.syncSelectedKey()
			m.loadFlows()
			m.lastView = ""
			return m, nil
		}

	case tea.MouseMsg:
		switch msg.Action {
		case tea.MouseActionPress:
			if msg.Button == tea.MouseButtonLeft {
				m.handleClick(msg.X, msg.Y)
				m.lastView = ""
			}
			if msg.Button == tea.MouseButtonWheelUp {
				if m.selected > 0 {
					m.selected--
					m.syncSelectedKey()
					m.loadFlows()
					m.lastView = ""
				}
			}
			if msg.Button == tea.MouseButtonWheelDown {
				if m.selected < len(m.apps)-1 {
					m.selected++
					m.syncSelectedKey()
					m.loadFlows()
					m.lastView = ""
				}
			}
		}
		return m, nil
	}
	return m, nil
}

func (m *Model) recomputeLayout() {
	// Fixed pane heights from terminal size — never depend on content length.
	helpH := 1
	headerH := 1
	m.graphH = max(6, m.height/5)
	m.detailH = max(5, m.height/6)
	m.appsH = m.height - headerH - m.graphH - m.detailH - helpH
	if m.appsH < 6 {
		m.appsH = 6
	}
}

func (m *Model) refresh() {
	if m.store == nil {
		return
	}
	m.store.TickSample(time.Now())
	raw := m.store.ListAppsByBandwidth(50)
	m.samples = m.store.Samples()
	m.adapters = m.store.ListAdapters()
	m.status = m.store.Snapshot()

	// EMA-smooth per-app rates so bars don't thrash every tick.
	const alpha = 0.35
	maxR := 0.0
	for _, a := range raw {
		r := a.RateInBps + a.RateOutBps
		prev := m.displayRates[a.Key]
		if prev <= 0 {
			prev = r
		} else {
			prev = alpha*r + (1-alpha)*prev
		}
		m.displayRates[a.Key] = prev
		if prev > maxR {
			maxR = prev
		}
	}
	// Smooth global max for bar scale (prevents full-width flash on spikes).
	if m.smoothMaxRate <= 0 {
		m.smoothMaxRate = maxR
	} else {
		m.smoothMaxRate = 0.2*maxR + 0.8*m.smoothMaxRate
	}
	if m.smoothMaxRate < 1 {
		m.smoothMaxRate = 1
	}
	// Drop stale EMA keys.
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
	// Restore selection by key so re-sorting doesn't jump the highlight.
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
			if m.selected >= len(m.apps) {
				m.selected = max(0, len(m.apps)-1)
			}
			m.syncSelectedKey()
		}
	} else {
		if m.selected >= len(m.apps) {
			m.selected = max(0, len(m.apps)-1)
		}
		m.syncSelectedKey()
	}
	if len(m.apps) == 0 {
		m.selected = 0
		m.selectedKey = ""
	}
	m.loadFlows()
	m.frameSeq++
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
	if m.height < 10 {
		return
	}
	// Regions match fixed layout: header(1) + graph + apps + detail + help
	graphEnd := 1 + m.graphH
	appsEnd := graphEnd + m.appsH
	switch {
	case y <= graphEnd:
		m.focus = paneGraph
	case y < appsEnd:
		m.focus = paneApps
		// Border takes 1 row; title 1 row inside pane.
		innerTop := graphEnd + 1 // after top border of apps pane
		row := y - innerTop - 1  // skip title line
		if row >= 0 && row < len(m.apps) {
			m.selected = row
			m.syncSelectedKey()
			m.loadFlows()
		}
	default:
		m.focus = paneDetail
		m.showDetail = true
	}
	_ = x
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
	if m.graphH == 0 {
		m.recomputeLayout()
	}

	w := m.width
	// Build fixed-height panes so JoinVertical never reflows when data changes.
	header := fitLine(m.renderHeader(), w)
	graphBody := m.renderGraphBody(max(1, m.graphH-2), max(1, w-4))
	appsBody := m.renderAppsBody(max(1, m.appsH-2), max(1, w-4))
	detailBody := m.renderDetailBody(max(1, m.detailH-2), max(1, w-4))
	help := fitLine(m.theme.Help.Render("↑↓/mouse select · tab panes · enter detail · esc back · q quit · theme: dark"), w)

	graph := m.theme.Border.Width(w - 2).Height(m.graphH - 2).Render(graphBody)
	apps := m.theme.Border.Width(w - 2).Height(m.appsH - 2).Render(appsBody)
	detail := m.theme.Border.Width(w - 2).Height(m.detailH - 2).Render(detailBody)

	// Force each section to exact visual height with pad/truncate.
	out := strings.Join([]string{
		padBlock(header, w, 1),
		padBlock(graph, w, m.graphH),
		padBlock(apps, w, m.appsH),
		padBlock(detail, w, m.detailH),
		padBlock(help, w, 1),
	}, "\n")

	m.lastView = out
	return out
}

func (m Model) renderHeader() string {
	state := "idle"
	if m.status.Running {
		state = "live"
	}
	switch m.status.Mode {
	case domain.ModeDemo:
		state = "demo"
	case domain.ModeStats:
		state = "stats"
	case domain.ModeLive:
		state = "live"
	}
	// Prefer a short, stable adapter summary (primary up iface + win count).
	adapterStr := summarizeAdapters(m.adapters)
	title := m.theme.Title.Render(" OpenWire ")
	meta := m.theme.Status.Render(fmt.Sprintf(" %s · %s · ↑%-10s ↓%-10s ",
		state, adapterStr,
		humanRate(m.status.TotalTxBps),
		humanRate(m.status.TotalRxBps),
	))
	if m.status.Message != "" {
		// Truncate long messages so header width stays stable.
		msg := m.status.Message
		if utf8.RuneCountInString(msg) > 36 {
			msg = truncate(msg, 36)
		}
		meta += m.theme.Warn.Render(" " + msg + " ")
	}
	if m.status.IsWSL2 {
		meta += m.theme.Muted.Render(" WSL2 ")
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, title, meta)
}

func summarizeAdapters(ads []domain.Adapter) string {
	if len(ads) == 0 {
		return "no adapters"
	}
	var linuxUp []string
	winN := 0
	for _, a := range ads {
		if a.Source == domain.AdapterSourceWindowsHost {
			if a.Up {
				winN++
			}
			continue
		}
		if a.Up && !a.Loopback {
			linuxUp = append(linuxUp, a.Name)
		}
	}
	parts := make([]string, 0, 3)
	if len(linuxUp) > 0 {
		// show at most 2 linux ifaces
		if len(linuxUp) > 2 {
			parts = append(parts, strings.Join(linuxUp[:2], ",")+"+")
		} else {
			parts = append(parts, strings.Join(linuxUp, ","))
		}
	}
	if winN > 0 {
		parts = append(parts, fmt.Sprintf("win×%d", winN))
	}
	if len(parts) == 0 {
		return ads[0].Name
	}
	return strings.Join(parts, " · ")
}

func (m Model) renderGraphBody(rows, cols int) string {
	focusMark := " "
	if m.focus == paneGraph {
		focusMark = "▶"
	}
	title := m.theme.Accent.Render(focusMark + " Bandwidth")
	lines := make([]string, 0, rows)
	lines = append(lines, fitLine(title, cols))
	if len(m.samples) == 0 {
		lines = append(lines, fitLine(m.theme.Muted.Render(" waiting for traffic…"), cols))
	} else {
		chartW := max(10, cols-2)
		chart := sparkline(m.samples, chartW, 1)
		lines = append(lines, fitLine(m.theme.Graph.Render(chart), cols))
		last := m.samples[len(m.samples)-1]
		stats := m.theme.Rx.Render(fmt.Sprintf("↓ %-10s", humanRate(last.RxBps))) + "  " +
			m.theme.Tx.Render(fmt.Sprintf("↑ %-10s", humanRate(last.TxBps))) + "  " +
			m.theme.Muted.Render(fmt.Sprintf("Σ %-10s", humanRate(last.TotalBps)))
		lines = append(lines, fitLine(stats, cols))
	}
	return joinFixed(lines, rows, cols)
}

func (m Model) renderAppsBody(rows, cols int) string {
	focusMark := " "
	if m.focus == paneApps {
		focusMark = "▶"
	}
	title := m.theme.Accent.Render(focusMark + " Applications (by usage)")
	lines := make([]string, 0, rows)
	lines = append(lines, fitLine(title, cols))
	if len(m.apps) == 0 {
		lines = append(lines, fitLine(m.theme.Muted.Render(" no apps yet"), cols))
		return joinFixed(lines, rows, cols)
	}

	// Fixed columns: mark(1) name(16) bar barW rate(10) up(9) down(9)
	barW := max(8, min(20, cols/5))
	nameW := 16
	// remaining room for numbers
	maxScale := m.smoothMaxRate
	if maxScale < 1 {
		maxScale = 1
	}

	// How many data rows fit under the title.
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
		// Fixed-width fields prevent horizontal jitter.
		line := fmt.Sprintf("%s %s %10s  ↑%-8s ↓%-8s",
			padRight(truncate(a.Name, nameW), nameW),
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
	lines := make([]string, 0, rows)
	lines = append(lines, fitLine(title, cols))
	if m.selected < 0 || m.selected >= len(m.apps) {
		lines = append(lines, fitLine(m.theme.Muted.Render(" select an application"), cols))
		return joinFixed(lines, rows, cols)
	}
	a := m.apps[m.selected]
	head := fmt.Sprintf("%s · pid %d · flows %d", truncate(a.Name, 20), a.PID, a.Flows)
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
		line := fmt.Sprintf("%s %s:%d ↔ %s:%d  in %s out %s",
			f.Key.Protocol, f.Key.SrcIP, f.Key.SrcPort, f.Key.DstIP, f.Key.DstPort,
			humanBytes(f.BytesIn), humanBytes(f.BytesOut),
		)
		lines = append(lines, fitLine(m.theme.Muted.Render(truncate(line, cols)), cols))
	}
	return joinFixed(lines, rows, cols)
}

// joinFixed pads or trims to exactly `rows` lines of width `cols`.
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

// padBlock ensures a block occupies exactly `rows` terminal rows of width `cols`.
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
	// Strip ANSI for length check via lipgloss, then pad.
	plain := lipgloss.Width(s)
	if plain == width {
		return s
	}
	if plain > width {
		// Truncate carefully: use runes of plain text when no styles, else pad left cut.
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
		row.WriteRune('▁') // stable filler instead of spaces (less visual jump)
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
