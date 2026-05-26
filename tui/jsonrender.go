package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func (t *jsonTree) View() string {
	if t.height <= 0 {
		return ""
	}
	var b strings.Builder
	rendered := 0
	for i := t.offset; i < len(t.visible) && rendered < t.height; i++ {
		for _, line := range t.renderLines(t.visible[i], i == t.cursor && t.focused) {
			if rendered >= t.height {
				break
			}
			b.WriteString(line)
			b.WriteByte('\n')
			rendered++
		}
	}
	for ; rendered < t.height; rendered++ {
		b.WriteByte('\n')
	}
	return b.String()
}

func (t *jsonTree) renderLines(vl *visibleLine, isCursor bool) []string {
	if vl.isSep {
		return []string{""}
	}

	n := vl.node
	indent := strings.Repeat("  ", vl.depth)

	marker := "  "
	if n.kind != kindScalar {
		if n.expanded {
			marker = "▾ "
		} else {
			marker = "▸ "
		}
	}

	// Depth-0 nodes (REQUEST / RESPONSE) render as bold section headers
	// with a fill line so they're visually distinct from payload fields.
	if vl.depth == 0 {
		label := strings.ToUpper(n.key)
		if isCursor {
			return []string{styleCursor.Width(t.width).Render(fitLine(marker+label, t.width))}
		}
		sectionStyle := styleSectionHeader
		if n.header {
			sectionStyle = styleHeaderSection
		}
		head := sectionStyle.Render(marker + label)
		fill := t.width - lipgloss.Width(head) - 1
		if fill < 0 {
			fill = 0
		}
		return []string{head + " " + lipgloss.NewStyle().Foreground(colBorder).Render(strings.Repeat("─", fill))}
	}

	// Build the summary suffix for collapsed containers.
	collapsedSuffix := func() string {
		switch {
		case !n.expanded && n.kind == kindObject:
			return fmt.Sprintf(" {%d}", len(n.children))
		case !n.expanded && n.kind == kindArray:
			return fmt.Sprintf(" [%d]", len(n.children))
		}
		return ""
	}

	if isCursor {
		if n.kind == kindScalar {
			prefix := indent + marker + n.key
			if n.key != "" {
				prefix += ": "
			}
			return t.wrapScalarLines(prefix, n.plainValue+n.suffix, lipgloss.NewStyle(), n.linkURL, true)
		}
		return []string{styleCursor.Width(t.width).Render(fitLine(indent+marker+n.key+collapsedSuffix(), t.width))}
	}

	var b strings.Builder
	b.WriteString(indent)
	b.WriteString(styleMarker.Render(marker))
	keyStyle := styleKey
	if n.header {
		keyStyle = styleHeaderKey
	}
	if n.dim {
		keyStyle = styleDim
	}
	// Emphasize keys that actually match the filter (vs. path ancestors kept
	// only to reach a match).
	if t.filterOn && t.filter != "" && n.key != "" && keyMatchesFilter(n.key, t.filter) {
		keyStyle = styleMatch
	}
	b.WriteString(keyStyle.Render(n.key))
	switch {
	case n.kind == kindScalar:
		if n.key != "" {
			b.WriteString(styleDim.Render(": "))
		}
		if lipgloss.Width(b.String()+n.value+n.suffix) <= t.width {
			b.WriteString(lipgloss.NewStyle().Foreground(n.scalarColor).Render(n.value))
			if n.suffix != "" {
				b.WriteString(styleDim.Render(n.suffix))
			}
			return []string{fitLine(b.String(), t.width)}
		}
		return t.wrapScalarLines(b.String(), n.plainValue+n.suffix, lipgloss.NewStyle().Foreground(n.scalarColor), n.linkURL, false)
	case !n.expanded:
		b.WriteString(styleDim.Render(collapsedSuffix()))
	}
	return []string{fitLine(b.String(), t.width)}
}

func (t *jsonTree) wrapScalarLines(prefix string, value string, valueStyle lipgloss.Style, linkURL string, cursor bool) []string {
	if t.width <= 0 {
		return []string{""}
	}
	prefixWidth := lipgloss.Width(prefix)
	baseIndent := min(prefixWidth, max(0, t.width-1))
	continuation := strings.Repeat(" ", baseIndent)
	lines := []string{}

	if prefixWidth >= t.width {
		lines = append(lines, fitLine(prefix, t.width))
		prefix = continuation
		prefixWidth = lipgloss.Width(prefix)
	}

	firstWidth := max(1, t.width-prefixWidth)
	restWidth := max(1, t.width-lipgloss.Width(continuation))
	chunks := wrapText(value, firstWidth, restWidth)
	if len(chunks) == 0 {
		chunks = []string{""}
	}

	for i, chunk := range chunks {
		linePrefix := continuation
		if i == 0 {
			linePrefix = prefix
		}
		renderedChunk := valueStyle.Render(chunk)
		if linkURL != "" {
			renderedChunk = hyperlink(linkURL, renderedChunk)
		}
		line := linePrefix + renderedChunk
		lines = append(lines, fitLine(line, t.width))
	}
	if cursor {
		for i, line := range lines {
			lines[i] = styleCursor.Width(t.width).Render(line)
		}
	}
	return lines
}

func wrapText(s string, firstWidth, restWidth int) []string {
	if s == "" {
		return nil
	}
	width := max(1, firstWidth)
	chunks := []string{}
	var b strings.Builder
	for _, r := range s {
		next := b.String() + string(r)
		if b.Len() > 0 && lipgloss.Width(next) > width {
			chunks = append(chunks, b.String())
			b.Reset()
			width = max(1, restWidth)
		}
		b.WriteRune(r)
	}
	if b.Len() > 0 {
		chunks = append(chunks, b.String())
	}
	return chunks
}
