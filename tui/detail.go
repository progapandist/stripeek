package tui

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/lipgloss"
)

func (m Model) detailHeaderLines(width int) []string {
	if !m.hasSel {
		return []string{fitLine(styleFaint.Render("no call selected"), width)}
	}
	c := m.selected

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

	statusStyle := styleOK
	if c.Status >= 400 {
		statusStyle = styleErr
	}
	status := statusStyle.Bold(true).Render(fmt.Sprintf("%d", c.Status))
	latency := styleDim.Render(fmt.Sprintf("%dms", c.Latency.Milliseconds()))
	hint := styleDim.Render("(h to toggle headers)")
	lines = append(lines, fitLine(status+"  "+latency+"  "+hint, width))

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
	return lines
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
