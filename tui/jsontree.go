package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/progapandist/stripeek/proxy"
)

type nodeKind int

const (
	kindScalar nodeKind = iota
	kindObject
	kindArray
)

// Node construction lives in jsonbuild.go, fuzzy filtering in jsonfilter.go,
// and rendering in jsonrender.go. Tree styles live in styles.go.

// jsonNode is one entry in the collapsible tree.
type jsonNode struct {
	key         string
	value       string // rendered value, may include OSC 8 hyperlink sequences
	plainValue  string // value without hyperlink, used on cursor line
	linkURL     string // optional OSC 8 target for clickable scalar values
	suffix      string // dim annotation appended after value (e.g., human timestamp)
	scalarColor lipgloss.TerminalColor
	dim         bool // true for null / empty-string values
	kind        nodeKind
	children    []*jsonNode
	expanded    bool
}

type visibleLine struct {
	node  *jsonNode
	depth int
	isSep bool // blank separator between root sections
}

// jsonTree is a scrollable, foldable view over a captured call's bodies.
type jsonTree struct {
	roots   []*jsonNode
	visible []*visibleLine
	cursor  int
	offset  int
	width   int
	height  int
	focused bool

	typing     bool   // true while editing the key filter query
	filterOn   bool   // true while a key filter is applied to a section
	filter     string // fuzzy query matched against key names
	filterRoot int    // index into roots of the filtered section (cursor's section)

	matchCache map[*jsonNode]bool // per-rebuild memo: node or a descendant matches
}

func (t *jsonTree) setCall(c proxy.Call) {
	t.roots = []*jsonNode{
		bodyRoot("request", c.ReqBody),
		bodyRoot("response", c.RespBody),
	}
	t.cursor = 0
	t.offset = 0
	t.typing = false
	t.filterOn = false
	t.filter = ""
	t.filterRoot = 0
	t.rebuild()
}

func (t *jsonTree) clear() {
	t.roots = nil
	t.visible = nil
	t.cursor = 0
	t.offset = 0
	t.typing = false
	t.filterOn = false
	t.filter = ""
	t.filterRoot = 0
}

// rebuild recomputes the flat list of visible lines from the root sections,
// honoring fold state and any active key filter.
func (t *jsonTree) rebuild() {
	t.visible = t.visible[:0]
	for i, r := range t.roots {
		if i > 0 {
			// Blank separator between REQUEST and RESPONSE sections.
			t.visible = append(t.visible, &visibleLine{isSep: true})
		}
		if t.filterOn && i == t.filterRoot {
			t.walkFilteredRoot(r)
		} else {
			t.walkNode(r, 0)
		}
	}
	if t.cursor >= len(t.visible) {
		t.cursor = len(t.visible) - 1
	}
	if t.cursor < 0 {
		t.cursor = 0
	}
	t.cursor = t.skipSepForward(t.cursor)
	t.clampOffset()
}

// walkNode appends a node and its descendants, honoring the expanded flag.
func (t *jsonTree) walkNode(n *jsonNode, depth int) {
	t.visible = append(t.visible, &visibleLine{node: n, depth: depth})
	if n.expanded {
		for _, c := range n.children {
			t.walkNode(c, depth+1)
		}
	}
}

func (t *jsonTree) current() *jsonNode {
	if len(t.visible) == 0 {
		return nil
	}
	vl := t.visible[t.cursor]
	if vl.isSep {
		return nil
	}
	return vl.node
}

func (t *jsonTree) Update(msg tea.Msg) {
	km, ok := msg.(tea.KeyMsg)
	if !ok {
		return
	}

	// While typing a key filter, every rune feeds the query so the view
	// narrows live. enter applies and exits typing; esc cancels the filter.
	if t.typing {
		switch km.String() {
		case "esc":
			t.clearFilter()
		case "enter":
			t.typing = false
		case "backspace", "ctrl+h":
			if r := []rune(t.filter); len(r) > 0 {
				t.setFilter(string(r[:len(r)-1]))
			}
		default:
			if km.Type == tea.KeyRunes {
				t.setFilter(t.filter + string(km.Runes))
			}
		}
		return
	}

	switch km.String() {
	case "/":
		if t.filterOn {
			t.typing = true // resume editing, keeping the query and scope
		} else {
			t.startFilter()
		}
	case "esc":
		if t.filterOn {
			t.clearFilter()
		}
	case "up", "k":
		t.move(-1)
	case "down", "j":
		t.move(1)
	case "right", "l":
		if n := t.current(); n != nil && n.kind != kindScalar {
			if !n.expanded {
				n.expanded = true
				t.rebuild()
			} else {
				t.move(1)
			}
		}
	case "left", "h":
		n := t.current()
		if n != nil && n.kind != kindScalar && n.expanded {
			n.expanded = false
			t.rebuild()
		} else {
			t.jumpToParent()
		}
	case "enter", " ":
		if n := t.current(); n != nil && n.kind != kindScalar {
			n.expanded = !n.expanded
			t.rebuild()
		}
	case "+", "=":
		t.setAll(true)
	case "-", "_":
		t.setAll(false)
	case "g", "home":
		t.cursor = t.skipSepForward(0)
		t.clampOffset()
	case "G", "end":
		t.cursor = t.skipSepBackward(len(t.visible) - 1)
		t.clampOffset()
	case "pgdown", "ctrl+d":
		t.move(t.pageStep())
	case "pgup", "ctrl+u":
		t.move(-t.pageStep())
	}
}

func (t *jsonTree) pageStep() int {
	if t.height <= 1 {
		return 1
	}
	return t.height / 2
}

func (t *jsonTree) move(d int) {
	pos := t.cursor + d
	if pos < 0 {
		pos = 0
	}
	if pos >= len(t.visible) {
		pos = len(t.visible) - 1
	}
	if d >= 0 {
		pos = t.skipSepForward(pos)
	} else {
		pos = t.skipSepBackward(pos)
	}
	t.cursor = pos
	t.clampOffset()
}

func (t *jsonTree) skipSepForward(pos int) int {
	for pos < len(t.visible)-1 && t.visible[pos].isSep {
		pos++
	}
	return pos
}

func (t *jsonTree) skipSepBackward(pos int) int {
	for pos > 0 && t.visible[pos].isSep {
		pos--
	}
	return pos
}

func (t *jsonTree) clampOffset() {
	if len(t.visible) == 0 {
		t.offset = 0
		return
	}
	if t.offset >= len(t.visible) {
		t.offset = len(t.visible) - 1
	}
	if t.cursor < t.offset {
		t.offset = t.cursor
	}
	if t.height > 0 {
		for t.offset < t.cursor && t.rowsBeforeCursor() >= t.height {
			t.offset++
		}
	}
	if t.offset < 0 {
		t.offset = 0
	}
}

func (t *jsonTree) rowsBeforeCursor() int {
	rows := 0
	for i := t.offset; i < t.cursor && i < len(t.visible); i++ {
		rows += t.renderedLineCount(i)
	}
	return rows
}

func (t *jsonTree) renderedLineCount(i int) int {
	if i < 0 || i >= len(t.visible) {
		return 0
	}
	return max(1, len(t.renderLines(t.visible[i], false)))
}

func (t *jsonTree) jumpToParent() {
	if len(t.visible) == 0 {
		return
	}
	d := t.visible[t.cursor].depth
	for i := t.cursor - 1; i >= 0; i-- {
		vl := t.visible[i]
		if !vl.isSep && vl.depth < d {
			t.cursor = i
			break
		}
	}
	t.clampOffset()
}

func (t *jsonTree) setAll(v bool) {
	var walk func(n *jsonNode)
	walk = func(n *jsonNode) {
		if n.kind != kindScalar {
			n.expanded = v
		}
		for _, c := range n.children {
			walk(c)
		}
	}
	for _, r := range t.roots {
		r.expanded = true // REQUEST/RESPONSE headers always stay visible
		for _, c := range r.children {
			walk(c)
		}
	}
	t.rebuild()
}
