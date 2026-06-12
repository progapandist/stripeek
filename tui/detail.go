package tui

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/lipgloss"

	"github.com/progapandist/stripeek/proxy"
)

func (m Model) detailHeaderLines(width int) []string {
	if !m.hasSel {
		return []string{fitLine(styleFaint.Render("no call selected"), width)}
	}
	c := m.selected

	var lines []string
	if c.IsWebhook && m.selWebhook.eventType != "" {
		lines = webhookTitleLines(m.selWebhook, width)
	} else {
		lines = methodPathLines(c, width)
	}

	statusStyle := styleOK
	if c.Status >= 400 {
		statusStyle = styleErr
	}
	status := statusStyle.Bold(true).Render(fmt.Sprintf("%d", c.Status))
	latency := styleDim.Render(fmt.Sprintf("%dms", c.Latency.Milliseconds()))
	hint := styleDim.Render("(h to toggle headers)")
	lines = append(lines, fitLine(status+"  "+latency+"  "+hint, width))

	mode := c.KeyMode
	if c.IsWebhook {
		// Webhooks carry no API key, so the badge reads the event's livemode.
		mode = m.selWebhook.livemode
	}
	if badge := modeBadge(mode); badge != "" {
		lines = append(lines, fitLine(badge, width))
	}

	if c.Group != nil {
		groupChunks := wrapHeaderText(c.Group.Name, width, max(1, width-2))
		if len(groupChunks) == 0 {
			groupChunks = []string{""}
		}
		for i, chunk := range groupChunks {
			prefix := ""
			if i > 0 {
				prefix = "  "
			}
			lines = append(lines, fitLine(prefix+groupStyle(c.Group).Render(chunk), width))
		}
	}

	// Operation hint: only when the selection has related calls/events and no
	// relation mode is active — so it never appears without webhook traffic.
	if m.relationOpID == 0 && m.selOpID != 0 {
		if n := m.opMemberCount(m.selOpID) - 1; n > 0 {
			hint := fmt.Sprintf("%d related · r focus · ctrl+r dim", n)
			lines = append(lines, fitLine(styleFaint.Render(hint), width))
		}
	}
	return lines
}

// methodPathLines renders the outbound header title: "METHOD path", with
// continuation lines indented past the method.
func methodPathLines(c proxy.Call, width int) []string {
	lines := []string{}
	method := lipgloss.NewStyle().Bold(true).Render(c.Method + " ")
	methodW := lipgloss.Width(method)
	pathChunks := wrapHeaderText(callDisplayPath(c), max(1, width-methodW), max(1, width-methodW))
	if len(pathChunks) == 0 {
		pathChunks = []string{""}
	}
	for i, chunk := range pathChunks {
		prefix := strings.Repeat(" ", methodW)
		if i == 0 {
			prefix = method
		}
		lines = append(lines, fitLine(prefix+chunk, width))
	}
	return lines
}

// webhookTitleLines renders the webhook header title: the event name in place
// of "METHOD path", with the evt_… id beneath it.
func webhookTitleLines(info webhookInfo, width int) []string {
	lines := []string{}
	for _, chunk := range wrapHeaderText(info.eventType, width, width) {
		lines = append(lines, fitLine(styleWebhook.Render(chunk), width))
	}
	if info.eventID != "" {
		lines = append(lines, fitLine(styleDim.Render(info.eventID), width))
	}
	return lines
}

// modeBadge renders the call's TEST/LIVE badge, inferred from the API key.
// Empty when the mode couldn't be determined.
func modeBadge(mode string) string {
	switch mode {
	case "test":
		return styleModeTest.Render("TEST")
	case "live":
		return styleModeLive.Render("LIVE")
	}
	return ""
}

func wrapHeaderText(s string, firstWidth, restWidth int) []string {
	if s == "" {
		return nil
	}
	width := max(1, firstWidth)
	restWidth = max(1, restWidth)
	chunks := []string{}
	for s != "" {
		if lipgloss.Width(s) <= width {
			chunks = append(chunks, s)
			break
		}
		cut := fittingPrefix(s, width)
		if preferred := preferredHeaderBreak(s[:cut], width); preferred > 0 {
			cut = preferred
		}
		chunks = append(chunks, strings.TrimRight(s[:cut], " "))
		s = strings.TrimLeft(s[cut:], " ")
		width = restWidth
	}
	return chunks
}

func fittingPrefix(s string, width int) int {
	cols := 0
	last := 0
	for i, r := range s {
		next := cols + lipgloss.Width(string(r))
		if next > width {
			break
		}
		cols = next
		last = i + len(string(r))
	}
	if last == 0 {
		_, size := utf8.DecodeRuneInString(s)
		return size
	}
	return last
}

func preferredHeaderBreak(s string, width int) int {
	minWidth := max(1, width/2)
	best := 0
	for i, r := range s {
		if !strings.ContainsRune(" /?&=-", r) {
			continue
		}
		cut := i
		if r != ' ' {
			cut = i + len(string(r))
		}
		if lipgloss.Width(strings.TrimRight(s[:cut], " ")) >= minWidth {
			best = cut
		}
	}
	return best
}
