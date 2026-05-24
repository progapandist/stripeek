package tui

import (
	"fmt"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/progapandist/stripeek/proxy"
)

// groupEntry is a row in the group navigator: a group and how many captured
// calls belong to it. A nil group is the "All requests" row.
type groupEntry struct {
	group *proxy.Group
	count int
}

func (m *Model) updateGroups(msg tea.KeyMsg) {
	switch msg.String() {
	case "up", "k":
		m.moveGroup(-1)
	case "down", "j":
		m.moveGroup(1)
	case "enter":
		m.applyGroupCursor()
		m.focused = focusList
		m.tree.focused = false
	case "esc":
		m.groupFilterID = ""
		m.groupCursor = 0
		m.focused = focusList
		m.rebuildList()
	case "g":
		m.toggleGroups()
	case "ctrl+g", "cmd+g":
		m.startGroup()
	}
}

func (m *Model) startGroup() {
	if m.groupMgr == nil {
		m.groupMgr = NewGroupManager(nil)
	}
	group := m.groupMgr.Start()
	m.groupsVisible = true
	m.groupFilterID = group.ID
	m.layout()
	m.rebuildList()
	m.selectGroup(group.ID)
}

func (m *Model) toggleGroups() {
	m.groupsVisible = !m.groupsVisible
	if !m.groupsVisible {
		m.groupFilterID = ""
		if m.focused == focusGroups {
			m.focused = focusList
		}
	}
	m.layout()
	m.rebuildList()
}

func (m *Model) moveGroup(delta int) {
	if len(m.groups) == 0 {
		return
	}
	m.groupCursor += delta
	if m.groupCursor < 0 {
		m.groupCursor = 0
	}
	if m.groupCursor >= len(m.groups) {
		m.groupCursor = len(m.groups) - 1
	}
	m.applyGroupCursor()
}

func (m *Model) applyGroupCursor() {
	if m.groupCursor < 0 || m.groupCursor >= len(m.groups) {
		m.groupFilterID = ""
	} else if m.groups[m.groupCursor].group == nil {
		m.groupFilterID = ""
	} else {
		m.groupFilterID = m.groups[m.groupCursor].group.ID
	}
	m.rebuildList()
}

func (m *Model) selectGroup(id string) {
	for i, g := range m.groups {
		if g.group != nil && g.group.ID == id {
			m.groupCursor = i
			return
		}
	}
	m.groupCursor = 0
}

// rebuildGroups recomputes the navigator rows from the captured calls: an
// "All requests" row followed by each group (newest first) with its call count.
func (m *Model) rebuildGroups() {
	counts := map[string]int{}
	byID := map[string]*proxy.Group{}
	for _, item := range m.allCalls {
		if item.call.Group == nil || item.call.Group.ID == "" {
			continue
		}
		counts[item.call.Group.ID]++
		if _, ok := byID[item.call.Group.ID]; !ok {
			byID[item.call.Group.ID] = cloneGroup(item.call.Group)
		}
	}
	if active := m.groupMgr.Current(); active != nil {
		if _, ok := byID[active.ID]; !ok {
			byID[active.ID] = active
		}
	}

	entries := make([]groupEntry, 0, len(byID))
	for id, g := range byID {
		entries = append(entries, groupEntry{group: g, count: counts[id]})
	}
	sort.SliceStable(entries, func(i, j int) bool {
		a := entries[i].group
		b := entries[j].group
		if !a.StartedAt.Equal(b.StartedAt) {
			return a.StartedAt.After(b.StartedAt)
		}
		return a.Name < b.Name
	})
	groups := append([]groupEntry{{count: len(m.allCalls)}}, entries...)
	m.groups = groups
	if m.groupCursor >= len(m.groups) {
		m.groupCursor = len(m.groups) - 1
	}
	if m.groupCursor < 0 {
		m.groupCursor = 0
	}
}

func (m Model) groupHeader(width int) string {
	label := styleSectionHeader.Render("GROUPS")
	fill := width - lipgloss.Width(label) - 1
	if fill < 0 {
		fill = 0
	}
	return label + " " + lipgloss.NewStyle().Foreground(colBorder).Render(strings.Repeat("─", fill))
}

func (m Model) groupLines(width, height int) []string {
	lines := make([]string, 0, height)
	start := 0
	if m.groupCursor >= height {
		start = m.groupCursor - height + 1
	}
	for i := start; i < len(m.groups) && len(lines) < height; i++ {
		entry := m.groups[i]
		cursor := i == m.groupCursor && m.focused == focusGroups
		name := "All requests"
		style := styleDim
		if entry.group != nil {
			name = entry.group.Name
			style = groupStyle(entry.group)
		}
		line := fmt.Sprintf("%s  %d", style.Render(name), entry.count)
		if entry.group != nil && entry.group.ID == m.groupFilterID {
			line += styleFaint.Render("  selected")
		}
		if cursor {
			line = styleCursor.Width(width).Render(stripANSIForCursor(name, entry.count))
		}
		lines = append(lines, fitLine(line, width))
	}
	for len(lines) < height {
		lines = append(lines, "")
	}
	return lines
}

func stripANSIForCursor(name string, count int) string {
	return fmt.Sprintf("%s  %d", name, count)
}

func (m Model) groupLabel(id string) string {
	for _, g := range m.groups {
		if g.group != nil && g.group.ID == id {
			return g.group.Name
		}
	}
	return "group"
}

func (m Model) groupCount(id string) int {
	for _, g := range m.groups {
		if g.group != nil && g.group.ID == id {
			return g.count
		}
	}
	return 0
}
