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

// Direction glyphs prefix every row so inbound webhooks read apart from
// outbound API traffic at a glance.
const (
	glyphOutbound = "▶"
	glyphInbound  = "◀"
)

// callItem is a captured call as a bubbles/list entry.
type callItem struct {
	call    proxy.Call
	id      uint64
	webhook webhookInfo // derived once from the event body; zero for outbound calls
}

// FilterValue is what the "/" list filter matches against: the event name for a
// webhook (so /sub finds customer.subscription.*), otherwise the request path.
func (c callItem) FilterValue() string {
	if c.call.IsWebhook && c.webhook.eventType != "" {
		return c.webhook.eventType
	}
	return callDisplayPath(c.call)
}

func (c callItem) statusToken(selected bool) string {
	sty := styleOK
	if c.call.Status >= 400 {
		sty = styleErr
	}
	if selected {
		sty = sty.Bold(true)
	}
	return sty.Render(fmt.Sprintf("%d", c.call.Status))
}

func (c callItem) timeLatency() string {
	return styleDim.Render(fmt.Sprintf("%s  %dms",
		c.call.Time.Format("15:04:05 MST"),
		c.call.Latency.Milliseconds()))
}

// renderRows lays a call out over the delegate's two rows: a direction glyph,
// then "METHOD path STATUS" (outbound) or "event.name STATUS" (webhook), with
// "time latency" below. The label is middle-truncated so the glyph and status
// stay visible and the metadata keeps its own dedicated line.
func (c callItem) renderRows(contentW int, selected bool) (string, string) {
	glyph, glyphSty := glyphOutbound, styleDirOut
	if c.call.IsWebhook {
		glyph, glyphSty = glyphInbound, styleDirIn
	}
	dir := glyphSty.Render(glyph)

	// Webhook rows with a known event read by event name and drop the method
	// token (it's always POST); everything else keeps "METHOD path".
	label := callDisplayPath(c.call)
	labelSty := lipgloss.NewStyle()
	showMethod := true
	if c.call.IsWebhook {
		labelSty = styleWebhook // webhook rows pop in magenta
		if c.webhook.eventType != "" {
			label = c.webhook.eventType
			showMethod = false
		}
	}
	if selected {
		labelSty = labelSty.Bold(true)
	}

	status := c.statusToken(selected)
	dirW := lipgloss.Width(dir) + 1       // glyph + separating space
	statusW := lipgloss.Width(status) + 1 // separating space + status

	prefix, prefixW := "", 0
	if showMethod {
		methodSty := methodStyle(c.call.Method)
		if selected {
			methodSty = methodSty.Bold(true)
		}
		method := methodSty.Render(c.call.Method)
		prefix = method + " "
		prefixW = lipgloss.Width(method) + 1 // method + separating space
	}

	availLabel := max(1, contentW-dirW-prefixW-statusW)
	label = truncateMiddle(label, availLabel)
	top := dir + " " + prefix + labelSty.Render(label) + " " + status
	return fitLine(top, contentW), fitLine(c.timeLatency(), contentW)
}

// truncateMiddle shortens s to fit width display columns, replacing the middle
// with an ellipsis so both the leading resource and trailing query survive.
func truncateMiddle(s string, width int) string {
	if width <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= width {
		return s
	}
	if width == 1 {
		return "…"
	}
	avail := width - 1 // reserve a column for the ellipsis
	leftW := avail / 2
	left := s[:fittingPrefix(s, leftW)]
	return left + "…" + suffixWithin(s, avail-leftW)
}

// suffixWithin returns the longest trailing run of s that fits in width columns.
func suffixWithin(s string, width int) string {
	if width <= 0 {
		return ""
	}
	runes := []rune(s)
	cols := 0
	i := len(runes)
	for i > 0 {
		w := lipgloss.Width(string(runes[i-1]))
		if cols+w > width {
			break
		}
		cols += w
		i--
	}
	return string(runes[i:])
}

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

	contentW := max(1, m.Width()-2)
	top, bottom := c.renderRows(contentW, selected)
	_, _ = fmt.Fprintf(w, "%s %s\n%s %s", selectedBorder, top, border, bottom)
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
		next, action := editFilterQuery(m.filter, msg)
		switch action {
		case editCommit:
			m.filtering = false
		case editCancel:
			m.filtering = false
			m.filter = ""
			m.rebuildList()
		case editChange:
			m.filter = next
			m.rebuildList()
		}
		return nil
	}

	switch {
	case matches(msg, keyInspect):
		m.syncTree()
		if m.hasSel {
			m.focused = focusDetail
			m.filtering = false
			m.tree.focused = true
		}
		return nil
	case matches(msg, keyFilter):
		m.filtering = true
		return nil
	case matches(msg, keyNewGroup):
		m.startGroup()
		return nil
	case matches(msg, keyToggleGroups):
		m.toggleGroups()
		return nil
	case matches(msg, keyDismiss):
		if m.filter != "" {
			m.filter = ""
			m.rebuildList()
		}
		return nil
	case matches(msg, keyTop):
		m.moveListTo(0)
		return nil
	case matches(msg, keyBottom):
		m.moveListTo(len(m.list.Items()) - 1)
		return nil
	case matches(msg, keyPageDn):
		m.moveListBy(m.listFullPageStep())
		return nil
	case matches(msg, keyPageUp):
		m.moveListBy(-m.listFullPageStep())
		return nil
	case matches(msg, keyHalfDn):
		m.moveListBy(m.listHalfPageStep())
		return nil
	case matches(msg, keyHalfUp):
		m.moveListBy(-m.listHalfPageStep())
		return nil
	}

	// up/down/j/k and other navigation fall through to the bubbles list.
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
		m.selWebhook = webhookInfo{}
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
	suffix := m.callProgressLabel() + m.droppedLabel()
	if m.groupFilterID != "" {
		label := m.groupLabel(m.groupFilterID)
		if m.filter != "" {
			return styleDim.Render(fmt.Sprintf("%d of %d in %s", shown, m.groupCount(m.groupFilterID), label)) + suffix
		}
		return styleDim.Render(fmt.Sprintf("%d in %s", shown, label)) + suffix
	}
	if m.filter != "" {
		return styleDim.Render(fmt.Sprintf("%d of %d requests", shown, total)) + suffix
	}
	return styleDim.Render(fmt.Sprintf("%d requests", shown)) + suffix
}

// droppedLabel warns when the proxy outran the TUI and shed captures. Empty in
// the normal case so it never disturbs the count line.
func (m Model) droppedLabel() string {
	if m.dropped <= 0 {
		return ""
	}
	return styleErr.Render(fmt.Sprintf("   ⚠ %d dropped", m.dropped))
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
