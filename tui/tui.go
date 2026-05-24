package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/progapandist/stripeek/proxy"
)

const DefaultMaxCalls = 100

// Styles and the color palette live in styles.go.

type callItem struct {
	call proxy.Call
	id   uint64
}

func (c callItem) Title() string {
	status := styleOK.Render(fmt.Sprintf("%d", c.call.Status))
	if c.call.Status >= 400 {
		status = styleErr.Render(fmt.Sprintf("%d", c.call.Status))
	}
	return fmt.Sprintf("%s %s %s", c.call.Method, callDisplayPath(c.call), status)
}

func (c callItem) Description() string {
	return styleDim.Render(fmt.Sprintf("%s  %dms",
		c.call.Time.Format("15:04:05 MST"),
		c.call.Latency.Milliseconds()))
}

func (c callItem) FilterValue() string { return callDisplayPath(c.call) }

// NewCallMsg carries a newly captured call into the TUI.
type NewCallMsg proxy.Call

// Model is the root Bubble Tea model.
type Model struct {
	list      list.Model
	tree      jsonTree
	allCalls  []callItem // all captured calls, newest first
	maxCalls  int
	nextID    uint64
	filter    string // path substring filter
	filtering bool   // true while typing a filter
	selected  proxy.Call
	hasSel    bool
	loadedID  uint64
	width     int
	height    int
	focused   string // "list" | "detail"
}

func New() Model {
	return NewWithMaxCalls(DefaultMaxCalls)
}

func NewWithMaxCalls(maxCalls int) Model {
	return NewWithCalls(maxCalls, nil)
}

func NewWithCalls(maxCalls int, calls []proxy.Call) Model {
	delegate := list.NewDefaultDelegate()
	delegate.SetSpacing(0)
	delegate.Styles.NormalTitle = lipgloss.NewStyle().Padding(0, 0, 0, 2)
	delegate.Styles.NormalDesc = lipgloss.NewStyle().Padding(0, 0, 0, 2).Foreground(colDim)
	selBar := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder(), false, false, false, true).
		BorderForeground(colAccent).
		Padding(0, 0, 0, 1)
	delegate.Styles.SelectedTitle = selBar.Foreground(colAccent)
	delegate.Styles.SelectedDesc = selBar.Foreground(colAccentSoft)
	delegate.Styles.DimmedTitle = delegate.Styles.NormalTitle
	delegate.Styles.DimmedDesc = delegate.Styles.NormalDesc
	l := list.New(nil, delegate, 0, 0)
	l.SetShowTitle(false)
	l.SetShowStatusBar(false)
	l.SetShowHelp(false)
	l.SetFilteringEnabled(false)

	m := Model{
		list:     l,
		maxCalls: maxCalls,
		focused:  "list",
	}
	for _, c := range calls {
		m.nextID++
		m.allCalls = append([]callItem{{call: c, id: m.nextID}}, m.allCalls...)
		if m.maxCalls > 0 && len(m.allCalls) > m.maxCalls {
			m.allCalls = m.allCalls[:m.maxCalls]
		}
	}
	m.rebuildList()
	return m
}

func (m Model) Init() tea.Cmd { return nil }

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.layout()

	case tea.KeyMsg:
		// ctrl+c always quits; q quits unless the user is typing (list filter
		// or inspector key search).
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "q":
			if !m.inputActive() {
				return m, tea.Quit
			}
		}

		// Tab always switches panes, cancelling any in-progress text input.
		if msg.String() == "tab" || msg.String() == "shift+tab" {
			if m.focused == "list" {
				m.focused = "detail"
				m.filtering = false
			} else {
				m.focused = "list"
				m.tree.clearFilter()
			}
			m.tree.focused = m.focused == "detail"
			return m, nil
		}
		// esc in the inspector returns to the list, unless a key filter is
		// active (then esc clears the filter — handled by the tree).
		if m.focused == "detail" && msg.String() == "esc" && !m.tree.typing && !m.tree.filterOn {
			m.focused = "list"
			m.tree.focused = false
			return m, nil
		}

		if m.focused == "list" {
			cmd = m.updateList(msg)
		} else {
			m.tree.Update(msg)
		}
		return m, cmd

	case NewCallMsg:
		m.nextID++
		item := callItem{call: proxy.Call(msg), id: m.nextID}
		m.allCalls = append([]callItem{item}, m.allCalls...)
		if m.maxCalls > 0 && len(m.allCalls) > m.maxCalls {
			m.allCalls = m.allCalls[:m.maxCalls]
		}
		m.rebuildList()
		return m, nil
	}

	return m, cmd
}

// inputActive reports whether a text-entry mode is capturing keystrokes, so
// global shortcuts like "q" don't fire mid-typing.
func (m Model) inputActive() bool {
	return (m.focused == "list" && m.filtering) || (m.focused == "detail" && m.tree.typing)
}

func (m *Model) updateList(msg tea.KeyMsg) tea.Cmd {
	if m.filtering {
		switch msg.String() {
		case "enter":
			m.filtering = false
		case "esc":
			m.filtering = false
			m.filter = ""
			m.rebuildList()
		case "backspace", "ctrl+h":
			runes := []rune(m.filter)
			if len(runes) > 0 {
				m.filter = string(runes[:len(runes)-1])
				m.rebuildList()
			}
		default:
			if msg.Type == tea.KeyRunes {
				m.filter += string(msg.Runes)
				m.rebuildList()
			}
		}
		return nil
	}

	switch msg.String() {
	case "enter":
		m.syncTree()
		if m.hasSel {
			m.focused = "detail"
			m.filtering = false
			m.tree.focused = true
		}
		return nil
	case "/":
		m.filtering = true
		return nil
	case "esc":
		if m.filter != "" {
			m.filter = ""
			m.rebuildList()
		}
		return nil
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	m.syncTree()
	return cmd
}

// rebuildList re-applies the current filter and restores the previous selection.
func (m *Model) rebuildList() {
	items := make([]list.Item, 0, len(m.allCalls))
	for _, c := range m.allCalls {
		if m.filter == "" || strings.Contains(c.FilterValue(), m.filter) {
			items = append(items, c)
		}
	}
	m.list.SetItems(items)
	// Restore the previously selected call by ID.
	if m.loadedID != 0 {
		for i, item := range items {
			if ci, ok := item.(callItem); ok && ci.id == m.loadedID {
				m.list.Select(i)
				break
			}
		}
	}
	m.syncTree()
}

// geometry derives all pane dimensions from the terminal size so layout() and
// View() never drift apart. A pane is: 2 border columns + 1 space of padding on
// each side, so usable content width is paneW-4. Vertically a pane is
// top-border + body + bottom-border, and a single footer row sits below both.
type geom struct {
	leftW, rightW int // outer pane widths (left + 1 gap + right == total)
	leftCW        int // left content width
	rightCW       int // right content width
	bodyN         int // body rows inside each pane (identical for both)
	listH         int // rows available to the call list
	treeH         int // rows available to the inspector tree
}

func (m Model) geometry() geom {
	g := geom{}
	g.leftW = m.width / 2
	g.rightW = m.width - g.leftW - 1 // 1-column gap between panes
	g.leftCW = max(1, g.leftW-4)
	g.rightCW = max(1, g.rightW-4)
	g.bodyN = max(1, m.height-3) // footer(1) + top+bottom border(2)
	g.listH = max(1, g.bodyN-3)  // filter + count + rule
	g.treeH = max(1, g.bodyN-2)  // header + rule
	return g
}

func (m *Model) layout() {
	if m.width == 0 {
		return
	}
	g := m.geometry()
	m.list.SetSize(g.leftCW, g.listH)
	m.tree.width = g.rightCW
	m.tree.height = g.treeH
	m.tree.clampOffset()
}

func (m *Model) syncTree() {
	sel, ok := m.list.SelectedItem().(callItem)
	if !ok {
		return
	}
	if sel.id == m.loadedID {
		return
	}
	m.loadedID = sel.id
	m.selected = sel.call
	m.hasSel = true
	m.tree.setCall(sel.call)
}

func callDisplayPath(c proxy.Call) string {
	if c.RequestURI != "" {
		return c.RequestURI
	}
	return c.Path
}

func (m Model) View() string {
	if m.width == 0 {
		return "loading…"
	}
	g := m.geometry()

	leftBody := []string{
		m.filterBar(),
		m.callCountLine(),
		rule(g.leftCW),
	}
	leftBody = append(leftBody, toLines(m.list.View(), g.listH)...)

	rightSubhead := rule(g.rightCW)
	if m.tree.typing || m.tree.filterOn {
		rightSubhead = m.inspectorFilterBar()
	}
	rightBody := []string{
		m.detailHeader(),
		rightSubhead,
	}
	rightBody = append(rightBody, toLines(m.tree.View(), g.treeH)...)

	// Pad both bodies to the same height so the panes' bottom borders align.
	n := max(len(leftBody), len(rightBody))
	leftBody = padSlice(leftBody, n)
	rightBody = padSlice(rightBody, n)

	left := frame("Calls", leftBody, g.leftCW, m.focused == "list")
	right := frame("Inspector", rightBody, g.rightCW, m.focused == "detail")

	body := lipgloss.JoinHorizontal(lipgloss.Top, left, " ", right)
	return lipgloss.JoinVertical(lipgloss.Left, body, m.helpBar())
}

// frame draws a rounded box around body lines with the title set into the top
// border (active panes glow in the accent color, inactive ones stay gray).
func frame(title string, body []string, contentW int, active bool) string {
	border := colBorder
	titleStyle := styleTitleInactive
	label := title
	if active {
		border = colAccent
		titleStyle = styleTitleActive
		label = strings.ToUpper(title)
	}
	bc := lipgloss.NewStyle().Foreground(border)
	iw := contentW + 2 // interior width: content + one space of padding per side

	// Top edge: ╭─ TITLE ───…───╮. Width must equal body rows (contentW+4) or
	// JoinHorizontal pads unevenly and the panes' bottoms drift apart.
	tl := titleStyle.Render(label)
	dashes := contentW - 1 - lipgloss.Width(tl)
	if dashes < 0 {
		dashes = 0
	}
	top := bc.Render("╭─ ") + tl + bc.Render(" "+strings.Repeat("─", dashes)+"╮")
	bottom := bc.Render("╰" + strings.Repeat("─", iw) + "╯")
	bar := bc.Render("│")

	var b strings.Builder
	b.WriteString(top)
	b.WriteByte('\n')
	for _, line := range body {
		b.WriteString(bar)
		b.WriteString(" ")
		b.WriteString(fitLine(line, contentW))
		b.WriteString(" ")
		b.WriteString(bar)
		b.WriteByte('\n')
	}
	b.WriteString(bottom)
	return b.String()
}

// rule is a faint horizontal divider used inside a pane.
func rule(width int) string {
	return lipgloss.NewStyle().Foreground(colBorder).Render(strings.Repeat("─", width))
}

// fitLine clamps s to exactly width visible columns, padding short lines so the
// right border stays aligned regardless of embedded ANSI styling.
func fitLine(s string, width int) string {
	s = lipgloss.NewStyle().MaxWidth(width).Render(s)
	if pad := width - lipgloss.Width(s); pad > 0 {
		s += strings.Repeat(" ", pad)
	}
	return s
}

// toLines splits a rendered view into exactly n lines (trailing padding trimmed
// then re-added) so a pane body always has the height the layout reserved.
func toLines(s string, n int) []string {
	raw := strings.Split(strings.TrimRight(s, "\n"), "\n")
	return padSlice(raw, n)
}

func padSlice(lines []string, n int) []string {
	for len(lines) < n {
		lines = append(lines, "")
	}
	if len(lines) > n {
		lines = lines[:n]
	}
	return lines
}

func (m Model) filterBar() string {
	switch {
	case m.filtering:
		return styleDim.Render("/ ") + styleFilterText.Render(m.filter) + styleAccentBlock.Render(" ")
	case m.filter != "":
		return styleDim.Render("/ ") + styleFilterText.Render(m.filter) + styleFaint.Render("   esc clears")
	default:
		return styleFaint.Render("/ filter by path")
	}
}

func (m Model) inspectorFilterBar() string {
	section := "keys"
	if m.tree.filterRoot < len(m.tree.roots) {
		section = strings.ToLower(m.tree.roots[m.tree.filterRoot].key) + " keys"
	}
	label := styleDim.Render("filter " + section + "  /")
	query := styleFilterText.Render(m.tree.filter)
	if m.tree.typing {
		return label + query + styleAccentBlock.Render(" ")
	}
	return label + query + styleFaint.Render("   esc clears")
}

func (m Model) callCountLine() string {
	shown := len(m.list.Items())
	total := len(m.allCalls)
	if m.filter != "" {
		return styleDim.Render(fmt.Sprintf("%d of %d requests", shown, total))
	}
	return styleDim.Render(fmt.Sprintf("%d requests", shown))
}

func (m Model) detailHeader() string {
	if !m.hasSel {
		return styleFaint.Render("no call selected")
	}
	c := m.selected
	statusStyle := styleOK
	if c.Status >= 400 {
		statusStyle = styleErr
	}
	return fmt.Sprintf("%s %s  %s  %s",
		lipgloss.NewStyle().Bold(true).Render(c.Method),
		callDisplayPath(c),
		statusStyle.Bold(true).Render(fmt.Sprintf("%d", c.Status)),
		styleDim.Render(fmt.Sprintf("%dms", c.Latency.Milliseconds())),
	)
}

func (m Model) helpBar() string {
	var pairs [][2]string
	switch {
	case m.focused == "list" && m.filtering:
		pairs = [][2]string{
			{"type", "filter path"},
			{"esc", "clear"},
			{"enter", "done"},
			{"tab", "inspect"},
		}
	case m.focused == "list":
		pairs = [][2]string{
			{"↑↓", "move"},
			{"enter", "inspect"},
			{"/", "filter"},
			{"tab", "switch"},
			{"q", "quit"},
		}
	case m.focused == "detail" && m.tree.typing:
		pairs = [][2]string{
			{"type", "filter keys"},
			{"enter", "apply"},
			{"esc", "cancel"},
		}
	default:
		// Inspector keeps the full key set whether or not a filter is applied;
		// the "esc clears" hint for an active filter lives in the filter bar.
		pairs = [][2]string{
			{"↑↓", "move"},
			{"←→", "fold"},
			{"space", "toggle"},
			{"/", "filter keys"},
			{"+/−", "expand/collapse"},
			{"esc", "back"},
		}
	}

	parts := make([]string, 0, len(pairs))
	for _, p := range pairs {
		parts = append(parts, styleHelpKey.Render(p[0])+" "+styleHelp.Render(p[1]))
	}
	sep := styleFaint.Render("   ")
	bar := " " + joinWith(parts, sep)
	return lipgloss.NewStyle().Width(m.width).MaxWidth(m.width).Render(bar)
}

func joinWith(parts []string, sep string) string {
	var b strings.Builder
	for i, p := range parts {
		if i > 0 {
			b.WriteString(sep)
		}
		b.WriteString(p)
	}
	return b.String()
}
