package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func (m Model) helpBar() string {
	var pairs [][2]string
	switch {
	case m.focused == focusList && m.filtering:
		pairs = [][2]string{
			{"type", "filter path"},
			{"esc", "clear"},
			{"enter", "done"},
			{"tab", "switch"},
		}
	case m.focused == focusList:
		pairs = [][2]string{
			{"↑↓", "move"},
			{"enter", "inspect"},
			{"/", "filter"},
			{"g", "groups"},
			{"ctrl+x", "clear"},
			{"tab", "switch"},
			{"?", "shortcuts"},
			{"q", "quit"},
		}
	case m.focused == focusGroups:
		pairs = [][2]string{
			{"↑↓", "group"},
			{"enter", "calls"},
			{"esc", "all"},
			{"ctrl+g", "new group"},
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
		pairs = [][2]string{
			{"↑↓", "move"},
			{"←→", "open/close"},
			{"+/-", "all"},
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
			left = append(left, renderShortcutSection(section, colW, 14)...)
		}
		right := renderShortcutSection(inspectorShortcuts(), colW, 14)
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
	for _, section := range shortcutSections() {
		if len(lines) > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, renderShortcutSection(section, contentW, 16)...)
	}
	return frame("Shortcuts", lines, contentW, true)
}

func renderShortcutSection(section shortcutSection, width, keyWidth int) []string {
	lines := []string{styleSectionHeader.Render(section.title)}
	for _, p := range section.pairs {
		key := styleHelpKey.Width(keyWidth).Render(p[0])
		lines = append(lines, fitLine(key+styleHelp.Render(p[1]), width))
	}
	return lines
}

type shortcutSection struct {
	title string
	pairs [][2]string
}

func shortcutSections() []shortcutSection {
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
			{"?", "toggle shortcuts"},
			{"ctrl+x", "clear history"},
			{"q", "quit"},
			{"ctrl+c", "quit now"},
		},
	}
}

func callsShortcuts() shortcutSection {
	return shortcutSection{
		title: "Calls",
		pairs: [][2]string{
			{"↑↓ / j k", "move request"},
			{"pgup/pgdn", "move page"},
			{"ctrl+b/f", "move page"},
			{"ctrl+u/d", "move half page"},
			{"home/end", "top/bottom"},
			{"t / b", "top/bottom"},
			{"enter", "inspect request"},
			{"/ / esc", "filter / clear"},
			{"g / ctrl+g", "groups / new group"},
		},
	}
}

func inspectorShortcuts() shortcutSection {
	return shortcutSection{
		title: "Inspector",
		pairs: [][2]string{
			{"↑↓ / j k", "move row"},
			{"pgup/pgdn", "move page"},
			{"ctrl+b/f", "move page"},
			{"ctrl+u/d", "move half page"},
			{"home/end", "top/bottom"},
			{"t / b", "top/bottom"},
			{"←→ / h l", "expand/collapse"},
			{"space/enter", "toggle container"},
			{"+ / -", "expand/collapse all"},
			{"/ / esc", "filter keys / clear"},
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
