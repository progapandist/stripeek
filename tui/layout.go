package tui

// geometry derives all pane dimensions from the terminal size so layout() and
// View() never drift apart. A pane is: 2 border columns + 1 space of padding on
// each side, so usable content width is paneW-4. Vertically a pane is
// top-border + body + bottom-border, and a single footer row sits below both.
type geom struct {
	leftW, rightW int // outer pane widths (left + 1 gap + right == total)
	leftCW        int // left content width
	rightCW       int // right content width
	bodyN         int // body rows inside each pane (identical for both)
	listH         int // rows available to the call list
	groupH        int // rows available to the group navigator
	treeH         int // rows available to the inspector tree
}

func (m Model) geometry() geom {
	g := geom{}
	g.leftW = m.width / 2
	g.rightW = m.width - g.leftW - 1 // 1-column gap between panes
	g.leftCW = max(1, g.leftW-4)
	g.rightCW = max(1, g.rightW-4)
	g.bodyN = max(1, m.height-3) // footer(1) + top+bottom border(2)
	if m.groupsVisible {
		available := max(1, g.bodyN-4) // filter + count + rule + group header
		g.groupH = max(1, available/2)
		g.listH = max(1, available-g.groupH)
	} else {
		g.listH = max(1, g.bodyN-3) // filter + count + rule
	}
	g.treeH = max(1, g.bodyN-4) // header + spacer + rule/filter + spacer
	return g
}

func (m *Model) layout() {
	if m.width == 0 {
		return
	}
	g := m.geometry()
	m.list.SetSize(g.leftCW, g.listH)
	m.tree.width = g.rightCW
	m.tree.height = g.treeH
	m.tree.clampOffset()
}
