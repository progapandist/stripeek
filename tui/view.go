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
	leftBody = append(leftBody, m.callListLines(g)...)
	if m.groupsVisible {
		leftBody = append(leftBody, "")
		leftBody = append(leftBody, m.groupHeader(g.leftCW))
		leftBody = append(leftBody, m.groupLines(g.leftCW, g.groupH)...)
	}

	rightBody := append([]string{}, m.detailHeaderLines(g.rightCW)...)
	if m.tree.typing || m.tree.filterOn {
		rightBody = append(rightBody, m.inspectorFilterBar())
	}
	rightBody = append(rightBody, m.inspectorPathBar())
	rightBody = append(rightBody, "")
	rightBody = append(rightBody, m.inspectorTreeLines(g)...)

	// Pad both bodies to the same height so the panes' bottom borders align.
	n := max(len(leftBody), len(rightBody))
	leftBody = padSlice(leftBody, n)
	rightBody = padSlice(rightBody, n)

	left := frame("Calls", leftBody, g.leftCW, m.focused == focusList || m.focused == focusGroups)
	right := frame("Inspector", rightBody, g.rightCW, m.focused == focusDetail)

	body := lipgloss.JoinHorizontal(lipgloss.Top, left, " ", right)
	if m.shortcuts {
		body = lipgloss.Place(m.width, max(1, m.height-1), lipgloss.Center, lipgloss.Center, m.shortcutsOverlay())
	}
	return lipgloss.JoinVertical(lipgloss.Left, body, m.helpBar())
}

func (m Model) inspectorFilterBar() string {
	label := styleDim.Render("filter keys  /")
	query := styleFilterText.Render(m.tree.filter)
	if m.tree.typing {
		return label + query + styleAccentBlock.Render(" ")
	}
	return label + query + styleFaint.Render("   esc clears")
}

func (m Model) inspectorPathBar() string {
	path := m.tree.currentPath()
	if path == "" {
		path = "-"
	}
	return styleFaint.Render("path ") + stylePath.Render(path)
}

func (m Model) callListLines(g geom) []string {
	return toLines(m.list.View(), g.listH)
}

func (m Model) inspectorTreeLines(g geom) []string {
	lines := toLines(m.tree.View(), g.treeH)
	marks := m.tree.scrollbarMarks()
	for i := range lines {
		mark := " "
		if i < len(marks) {
			mark = marks[i]
		}
		lines[i] = fitLine(lines[i], m.tree.width) + " " + mark
	}
	return lines
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
