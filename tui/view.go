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
		leftBody = append(leftBody, "")
		leftBody = append(leftBody, m.groupHeader(g.leftCW))
		leftBody = append(leftBody, m.groupLines(g.leftCW, g.groupH)...)
	}

	rightBody := append([]string{}, m.detailHeaderLines(g.rightCW)...)
	rightBody = append(rightBody, "")
	if m.tree.typing || m.tree.filterOn {
		rightBody = append(rightBody, m.inspectorFilterBar(), "")
	}
	rightBody = append(rightBody, toLines(m.tree.View(), g.treeH)...)

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
			{"tab", "switch"},
			{"?", "shortcuts"},
			{"q", "quit"},
		}
	case m.focused == focusGroups:
		pairs = [][2]string{
			{"↑↓", "group"},
			{"enter", "calls"},
			{"esc", "all"},
			{"tab", "switch"},
			{"?", "shortcuts"},
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
			{"pgup/dn", "page"},
			{"/", "filter keys"},
			{"?", "shortcuts"},
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

func (m Model) shortcutsOverlay() string {
	width := min(96, max(36, m.width-8))
	contentW := max(1, width-4)

	if contentW >= 64 {
		gap := 3
		colW := (contentW - gap) / 2
		left := []string{}
		for _, section := range []shortcutSection{globalShortcuts(), callsShortcuts(), groupsShortcuts()} {
			if len(left) > 0 {
				left = append(left, "")
			}
			left = append(left, renderShortcutSection(section, colW)...)
		}
		right := renderShortcutSection(inspectorShortcuts(), colW)
		lines := make([]string, max(len(left), len(right)))
		for i := range lines {
			l := ""
			if i < len(left) {
				l = left[i]
			}
			r := ""
			if i < len(right) {
				r = right[i]
			}
			lines[i] = fitLine(fitLine(l, colW)+strings.Repeat(" ", gap)+fitLine(r, colW), contentW)
		}
		return frame("Shortcuts", lines, contentW, true)
	}

	lines := []string{}
	for _, section := range m.shortcutSections() {
		if len(lines) > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, styleSectionHeader.Render(section.title))
		for _, p := range section.pairs {
			key := styleHelpKey.Width(16).Render(p[0])
			lines = append(lines, fitLine(key+styleHelp.Render(p[1]), contentW))
		}
	}
	return frame("Shortcuts", lines, contentW, true)
}

func renderShortcutSection(section shortcutSection, width int) []string {
	lines := []string{styleSectionHeader.Render(section.title)}
	for _, p := range section.pairs {
		key := styleHelpKey.Width(14).Render(p[0])
		lines = append(lines, fitLine(key+styleHelp.Render(p[1]), width))
	}
	return lines
}

type shortcutSection struct {
	title string
	pairs [][2]string
}

func (m Model) shortcutSections() []shortcutSection {
	return []shortcutSection{
		globalShortcuts(),
		callsShortcuts(),
		inspectorShortcuts(),
		groupsShortcuts(),
	}
}

func globalShortcuts() shortcutSection {
	return shortcutSection{
		title: "Global",
		pairs: [][2]string{
			{"tab", "switch panes"},
			{"shift+tab", "switch back"},
			{"?", "open/close shortcuts"},
			{"q", "quit"},
			{"ctrl+c", "quit now"},
		},
	}
}

func callsShortcuts() shortcutSection {
	return shortcutSection{
		title: "Calls",
		pairs: [][2]string{
			{"↑↓ / j k", "move one request"},
			{"pgup/pgdn", "move one page"},
			{"ctrl+b/f", "move one page"},
			{"ctrl+u/d", "move half page"},
			{"home/end", "jump top/bottom"},
			{"t / b", "jump top/bottom"},
			{"enter", "inspect selected"},
			{"/ / esc", "filter / clear filter"},
			{"g / ctrl+g", "groups / new group"},
		},
	}
}

func inspectorShortcuts() shortcutSection {
	return shortcutSection{
		title: "Inspector",
		pairs: [][2]string{
			{"↑↓ / j k", "move one row"},
			{"pgup/pgdn", "move one page"},
			{"ctrl+b/f", "move one page"},
			{"ctrl+u/d", "move half page"},
			{"home/end", "jump top/bottom"},
			{"t / b", "jump top/bottom"},
			{"←→ / h l", "fold / enter"},
			{"space/enter", "toggle container"},
			{"+ / -", "expand / collapse all"},
			{"/ / esc", "filter keys / back"},
		},
	}
}

func groupsShortcuts() shortcutSection {
	return shortcutSection{
		title: "Groups",
		pairs: [][2]string{
			{"↑↓ / j k", "move group"},
			{"enter", "show group calls"},
			{"esc", "show all requests"},
			{"g", "hide groups"},
			{"ctrl+g", "start new group"},
		},
	}
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
