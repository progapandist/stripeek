package tui

import (
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/progapandist/stripeek/proxy"
)

// callItem is a captured call as a bubbles/list entry.
type callItem struct {
	call proxy.Call
	id   uint64
}

func (c callItem) Title() string { return c.title() }

func (c callItem) title() string {
	status := styleOK.Render(fmt.Sprintf("%d", c.call.Status))
	if c.call.Status >= 400 {
		status = styleErr.Render(fmt.Sprintf("%d", c.call.Status))
	}
	return fmt.Sprintf("%s %s %s", c.call.Method, callDisplayPath(c.call), status)
}

func (c callItem) Description() string { return c.description() }

func (c callItem) description() string {
	return styleDim.Render(fmt.Sprintf("%s  %dms",
		c.call.Time.Format("15:04:05 MST"),
		c.call.Latency.Milliseconds()))
}

func (c callItem) FilterValue() string { return callDisplayPath(c.call) }

// callDelegate renders each call as a two-line entry with a group/selection
// border in the gutter.
type callDelegate struct{}

func (callDelegate) Height() int                         { return 2 }
func (callDelegate) Spacing() int                        { return 0 }
func (callDelegate) Update(tea.Msg, *list.Model) tea.Cmd { return nil }

func (callDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	c, ok := item.(callItem)
	if !ok || m.Width() <= 0 {
		return
	}

	selected := index == m.Index()
	border := callBorder(c.call.Group, false)
	selectedBorder := callBorder(c.call.Group, selected)
	title := c.title()
	desc := c.description()
	if selected {
		title = lipgloss.NewStyle().Bold(true).Render(title)
	}

	contentW := max(1, m.Width()-2)
	title = fitLine(title, contentW)
	desc = fitLine(desc, contentW)
	_, _ = fmt.Fprintf(w, "%s %s\n%s %s", selectedBorder, title, border, desc)
}

func callBorder(group *proxy.Group, selected bool) string {
	if group != nil {
		if selected {
			return groupStyle(group).Bold(true).Render("█")
		}
		return groupStyle(group).Render("▌")
	}
	if selected {
		return lipgloss.NewStyle().Foreground(colAccent).Render("█")
	}
	return " "
}

func callDisplayPath(c proxy.Call) string {
	if c.RequestURI != "" {
		return c.RequestURI
	}
	return c.Path
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
			m.focused = focusDetail
			m.filtering = false
			m.tree.focused = true
		}
		return nil
	case "/":
		m.filtering = true
		return nil
	case "ctrl+g", "cmd+g":
		m.startGroup()
		return nil
	case "g":
		m.toggleGroups()
		return nil
	case "esc":
		if m.filter != "" {
			m.filter = ""
			m.rebuildList()
		}
		return nil
	case "t", "home":
		m.moveListTo(0)
		return nil
	case "b", "end":
		m.moveListTo(len(m.list.Items()) - 1)
		return nil
	case "pgdown", "ctrl+f":
		m.moveListBy(m.listFullPageStep())
		return nil
	case "pgup", "ctrl+b":
		m.moveListBy(-m.listFullPageStep())
		return nil
	case "ctrl+d":
		m.moveListBy(m.listHalfPageStep())
		return nil
	case "ctrl+u":
		m.moveListBy(-m.listHalfPageStep())
		return nil
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	m.syncTree()
	return cmd
}

func (m *Model) moveListBy(delta int) {
	m.moveListTo(m.list.Index() + delta)
}

func (m *Model) moveListTo(index int) {
	items := m.list.Items()
	if len(items) == 0 {
		return
	}
	if index < 0 {
		index = 0
	}
	if index >= len(items) {
		index = len(items) - 1
	}
	m.list.Select(index)
	m.syncTree()
}

func (m Model) listHalfPageStep() int {
	return max(1, m.visibleListItems()/2)
}

func (m Model) listFullPageStep() int {
	return max(1, m.visibleListItems()-1)
}

func (m Model) visibleListItems() int {
	return max(1, m.list.Height()/callDelegate{}.Height())
}

// rebuildList re-applies the current filter and restores the previous selection.
func (m *Model) rebuildList() {
	m.rebuildGroups()
	items := make([]list.Item, 0, len(m.allCalls))
	for _, c := range m.allCalls {
		if m.groupFilterID != "" && (c.call.Group == nil || c.call.Group.ID != m.groupFilterID) {
			continue
		}
		if m.filter == "" || strings.Contains(c.FilterValue(), m.filter) {
			items = append(items, c)
		}
	}
	m.list.SetItems(items)
	if len(items) == 0 {
		m.loadedID = 0
		m.hasSel = false
		m.selected = proxy.Call{}
		m.tree.clear()
		return
	}
	// Restore the previously selected call by ID.
	restored := false
	if m.loadedID != 0 {
		for i, item := range items {
			if ci, ok := item.(callItem); ok && ci.id == m.loadedID {
				m.list.Select(i)
				restored = true
				break
			}
		}
	}
	if !restored {
		m.list.Select(0)
		m.loadedID = 0
	}
	m.syncTree()
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

func (m Model) callCountLine() string {
	shown := len(m.list.Items())
	total := len(m.allCalls)
	progress := m.callProgressLabel()
	if m.groupFilterID != "" {
		label := m.groupLabel(m.groupFilterID)
		if m.filter != "" {
			return styleDim.Render(fmt.Sprintf("%d of %d in %s", shown, m.groupCount(m.groupFilterID), label)) + progress
		}
		return styleDim.Render(fmt.Sprintf("%d in %s", shown, label)) + progress
	}
	if m.filter != "" {
		return styleDim.Render(fmt.Sprintf("%d of %d requests", shown, total)) + progress
	}
	return styleDim.Render(fmt.Sprintf("%d requests", shown)) + progress
}

func (m Model) callProgressLabel() string {
	items := len(m.list.Items())
	if items <= m.visibleListItems() {
		return ""
	}
	index := m.list.Index()
	if index < 0 {
		index = 0
	}
	if index >= items {
		index = items - 1
	}
	direction := "↓"
	if index > 0 && index < items-1 {
		direction = "↑↓"
	} else if index == items-1 {
		direction = "↑"
	}
	return styleFaint.Render(fmt.Sprintf("   %d/%d %s", index+1, items, direction))
}
