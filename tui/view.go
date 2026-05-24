package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

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
	if m.groupsVisible {
		leftBody = append(leftBody, m.groupHeader(g.leftCW))
		leftBody = append(leftBody, m.groupLines(g.leftCW, g.groupH)...)
	}

	rightSubhead := rule(g.rightCW)
	if m.tree.typing || m.tree.filterOn {
		rightSubhead = m.inspectorFilterBar()
	}
	rightBody := []string{
		m.detailHeader(),
		"",
		rightSubhead,
		"",
	}
	rightBody = append(rightBody, toLines(m.tree.View(), g.treeH)...)

	// Pad both bodies to the same height so the panes' bottom borders align.
	n := max(len(leftBody), len(rightBody))
	leftBody = padSlice(leftBody, n)
	rightBody = padSlice(rightBody, n)

	left := frame("Calls", leftBody, g.leftCW, m.focused == focusList || m.focused == focusGroups)
	right := frame("Inspector", rightBody, g.rightCW, m.focused == focusDetail)

	body := lipgloss.JoinHorizontal(lipgloss.Top, left, " ", right)
	return lipgloss.JoinVertical(lipgloss.Left, body, m.helpBar())
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

func (m Model) helpBar() string {
	var pairs [][2]string
	switch {
	case m.focused == focusList && m.filtering:
		pairs = [][2]string{
			{"type", "filter path"},
			{"esc", "clear"},
			{"enter", "done"},
			{"tab", "inspect"},
		}
	case m.focused == focusList:
		pairs = [][2]string{
			{"↑↓", "move"},
			{"enter", "inspect"},
			{"/", "filter"},
			{"ctrl+g", "new group"},
			{"g", "groups"},
			{"tab", "switch"},
			{"q", "quit"},
		}
	case m.focused == focusGroups:
		pairs = [][2]string{
			{"↑↓", "group"},
			{"enter", "calls"},
			{"esc", "all"},
			{"ctrl+g", "new group"},
			{"g", "hide"},
			{"tab", "switch"},
		}
	case m.focused == focusDetail && m.tree.typing:
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
