package tui

import (
	"fmt"
	"strings"
	"time"

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

	focus      pane
	selected   int
	showDetail bool

	apps     []domain.AppUsage
	samples  []domain.BandwidthSample
	adapters []domain.Adapter
	status   domain.Status
	flows    []domain.Flow

	ready bool
}

// NewModel constructs the TUI model bound to a store.
func NewModel(store *memory.Store, themeName string) Model {
	_ = themeName // only dark for v0.0.1
	return Model{
		store: store,
		theme: DarkTheme(),
		focus: paneApps,
	}
}

type tickMsg time.Time

func tickCmd() tea.Cmd {
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
			return m, nil
		case "shift+tab":
			m.focus = (m.focus + 2) % 3
			return m, nil
		case "up", "k":
			if m.focus == paneApps || m.focus == paneDetail {
				if m.selected > 0 {
					m.selected--
				}
				m.loadFlows()
			}
			return m, nil
		case "down", "j":
			if m.focus == paneApps || m.focus == paneDetail {
				if m.selected < len(m.apps)-1 {
					m.selected++
				}
				m.loadFlows()
			}
			return m, nil
		case "enter":
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
			m.loadFlows()
			return m, nil
		case "end":
			if len(m.apps) > 0 {
				m.selected = len(m.apps) - 1
			}
			m.loadFlows()
			return m, nil
		}

	case tea.MouseMsg:
		switch msg.Action {
		case tea.MouseActionPress:
			if msg.Button == tea.MouseButtonLeft {
				m.handleClick(msg.X, msg.Y)
			}
			if msg.Button == tea.MouseButtonWheelUp {
				if m.selected > 0 {
					m.selected--
					m.loadFlows()
				}
			}
			if msg.Button == tea.MouseButtonWheelDown {
				if m.selected < len(m.apps)-1 {
					m.selected++
					m.loadFlows()
				}
			}
		}
		return m, nil
	}
	return m, nil
}

func (m *Model) refresh() {
	if m.store == nil {
		return
	}
	m.store.TickSample(time.Now())
	m.apps = m.store.ListAppsByBandwidth(50)
	m.samples = m.store.Samples()
	m.adapters = m.store.ListAdapters()
	m.status = m.store.Snapshot()
	if m.selected >= len(m.apps) && len(m.apps) > 0 {
		m.selected = len(m.apps) - 1
	}
	if len(m.apps) == 0 {
		m.selected = 0
	}
	m.loadFlows()
}

func (m *Model) loadFlows() {
	if m.store == nil || m.selected < 0 || m.selected >= len(m.apps) {
		m.flows = nil
		return
	}
	m.flows = m.store.ListFlowsForApp(m.apps[m.selected].Key, 8)
}

func (m *Model) handleClick(x, y int) {
	// Rough regions based on layout: header 1, graph ~height/4, apps middle, detail bottom, help last.
	if m.height < 10 {
		return
	}
	graphEnd := 1 + max(4, m.height/5)
	appsEnd := m.height - max(4, m.height/5) - 1
	switch {
	case y <= graphEnd:
		m.focus = paneGraph
	case y < appsEnd:
		m.focus = paneApps
		// map y to row inside apps pane (header line + rows)
		row := y - graphEnd - 2
		if row >= 0 && row < len(m.apps) {
			m.selected = row
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

	header := m.renderHeader()
	graph := m.renderGraph()
	apps := m.renderApps()
	detail := m.renderDetail()
	help := m.theme.Help.Render("↑↓/mouse select · tab panes · enter detail · esc back · q quit · theme: dark")

	// Compute pane heights.
	helpH := 1
	headerH := 1
	graphH := max(5, m.height/5)
	detailH := max(4, m.height/6)
	appsH := m.height - headerH - graphH - detailH - helpH - 1
	if appsH < 5 {
		appsH = 5
	}

	graph = m.theme.Border.Width(m.width - 2).Height(graphH - 2).Render(graph)
	apps = m.theme.Border.Width(m.width - 2).Height(appsH - 2).Render(apps)
	detail = m.theme.Border.Width(m.width - 2).Height(detailH - 2).Render(detail)

	return lipgloss.JoinVertical(lipgloss.Left,
		header,
		graph,
		apps,
		detail,
		help,
	)
}

func (m Model) renderHeader() string {
	mode := string(m.status.Mode)
	if mode == "" {
		mode = "unknown"
	}
	state := "idle"
	if m.status.Running {
		state = "live"
	}
	if m.status.Mode == domain.ModeDemo {
		state = "demo"
	}
	parts := make([]string, 0, 8)
	for _, a := range m.adapters {
		label := a.Name
		if a.Source == domain.AdapterSourceWindowsHost {
			label = "win:" + a.Name
		}
		parts = append(parts, label)
	}
	adapterStr := strings.Join(parts, " · ")
	if adapterStr == "" {
		adapterStr = "no adapters"
	}
	title := m.theme.Title.Render(" OpenWire ")
	meta := m.theme.Status.Render(fmt.Sprintf(" %s · %s · ↑ %s  ↓ %s ",
		state, adapterStr, humanRate(m.status.TotalTxBps), humanRate(m.status.TotalRxBps),
	))
	if m.status.Message != "" {
		meta += m.theme.Warn.Render(" "+m.status.Message+" ")
	}
	if m.status.IsWSL2 {
		meta += m.theme.Muted.Render(" WSL2 ")
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, title, meta)
}

func (m Model) renderGraph() string {
	focusMark := " "
	if m.focus == paneGraph {
		focusMark = "▶"
	}
	title := m.theme.Accent.Render(focusMark + " Bandwidth (last samples)")
	if len(m.samples) == 0 {
		return title + "\n" + m.theme.Muted.Render(" waiting for traffic…")
	}
	width := max(20, m.width-6)
	height := 3
	chart := sparkline(m.samples, width, height)
	last := m.samples[len(m.samples)-1]
	stats := m.theme.Rx.Render(fmt.Sprintf("↓ %s", humanRate(last.RxBps))) + "  " +
		m.theme.Tx.Render(fmt.Sprintf("↑ %s", humanRate(last.TxBps))) + "  " +
		m.theme.Muted.Render(fmt.Sprintf("total %s", humanRate(last.TotalBps)))
	return title + "\n" + m.theme.Graph.Render(chart) + "\n" + stats
}

func (m Model) renderApps() string {
	focusMark := " "
	if m.focus == paneApps {
		focusMark = "▶"
	}
	title := m.theme.Accent.Render(focusMark + " Applications (by usage)")
	if len(m.apps) == 0 {
		return title + "\n" + m.theme.Muted.Render(" no apps yet")
	}
	var b strings.Builder
	b.WriteString(title)
	b.WriteByte('\n')
	barW := max(8, min(24, m.width/4))
	maxRate := 0.0
	for _, a := range m.apps {
		r := a.RateInBps + a.RateOutBps
		if r > maxRate {
			maxRate = r
		}
	}
	if maxRate <= 0 {
		maxRate = 1
	}
	for i, a := range m.apps {
		rate := a.RateInBps + a.RateOutBps
		bar := rateBar(rate/maxRate, barW)
		line := fmt.Sprintf(" %-16s %s %10s  ↑%-8s ↓%-8s",
			truncate(a.Name, 16),
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
		b.WriteString(line)
		b.WriteByte('\n')
		if i >= 40 {
			break
		}
	}
	return strings.TrimRight(b.String(), "\n")
}

func (m Model) renderDetail() string {
	focusMark := " "
	if m.focus == paneDetail {
		focusMark = "▶"
	}
	title := m.theme.Accent.Render(focusMark + " Detail")
	if m.selected < 0 || m.selected >= len(m.apps) {
		return title + "\n" + m.theme.Muted.Render(" select an application")
	}
	a := m.apps[m.selected]
	head := fmt.Sprintf(" %s · pid %d · flows %d", a.Name, a.PID, a.Flows)
	if a.Path != "" {
		head += " · " + truncate(a.Path, max(10, m.width/3))
	}
	var b strings.Builder
	b.WriteString(title)
	b.WriteByte('\n')
	b.WriteString(m.theme.App.Render(head))
	b.WriteByte('\n')
	if len(m.flows) == 0 {
		b.WriteString(m.theme.Muted.Render(" no flows"))
		return b.String()
	}
	for _, f := range m.flows {
		line := fmt.Sprintf(" %s %s:%d ↔ %s:%d  in %s out %s",
			f.Key.Protocol, f.Key.SrcIP, f.Key.SrcPort, f.Key.DstIP, f.Key.DstPort,
			humanBytes(f.BytesIn), humanBytes(f.BytesOut),
		)
		b.WriteString(m.theme.Muted.Render(truncate(line, m.width-6)))
		b.WriteByte('\n')
	}
	return strings.TrimRight(b.String(), "\n")
}

func sparkline(samples []domain.BandwidthSample, width, height int) string {
	if width < 4 {
		width = 4
	}
	// Use last `width` samples.
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
	// Pad if fewer samples than width.
	for row.Len() < width {
		row.WriteRune(' ')
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
		return fmt.Sprintf("%d B", n)
	}
	div, exp := uint64(unit), 0
	for v := n / unit; v >= unit; v /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(n)/float64(div), "KMGTPE"[exp])
}

func truncate(s string, n int) string {
	if n <= 0 {
		return ""
	}
	if len(s) <= n {
		return s
	}
	if n <= 1 {
		return s[:n]
	}
	return s[:n-1] + "…"
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
