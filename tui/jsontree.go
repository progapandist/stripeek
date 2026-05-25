package tui

import (
	"strconv"

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

// Node construction lives in jsonbuild.go, key filtering in jsonfilter.go, and
// rendering in jsonrender.go. Tree styles live in styles.go.

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

	typing   bool   // true while editing the key filter query
	filterOn bool   // true while a key filter is applied
	filter   string // query matched against key names

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
		if t.filterOn {
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
	case "t", "home":
		t.cursor = t.skipSepForward(0)
		t.clampOffset()
	case "b", "end":
		t.cursor = t.skipSepBackward(len(t.visible) - 1)
		t.clampOffset()
	case "pgdown", "ctrl+f":
		t.move(t.fullPageStep())
	case "pgup", "ctrl+b":
		t.move(-t.fullPageStep())
	case "ctrl+d":
		t.move(t.halfPageStep())
	case "ctrl+u":
		t.move(-t.halfPageStep())
	}
}

func (t *jsonTree) halfPageStep() int {
	if t.height <= 1 {
		return 1
	}
	return t.height / 2
}

func (t *jsonTree) fullPageStep() int {
	return max(1, t.height-1)
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
		for t.offset > 0 && t.renderedRowsFromOffset(t.offset) < t.height && t.cursorFitsFrom(t.offset-1) {
			t.offset--
		}
	}
	if t.offset < 0 {
		t.offset = 0
	}
}

func (t *jsonTree) rowsBeforeCursor() int {
	return t.rowsBeforeCursorFrom(t.offset)
}

func (t *jsonTree) rowsBeforeCursorFrom(offset int) int {
	rows := 0
	for i := offset; i < t.cursor && i < len(t.visible); i++ {
		rows += t.renderedLineCount(i)
	}
	return rows
}

func (t *jsonTree) cursorFitsFrom(offset int) bool {
	if t.height <= 0 {
		return true
	}
	return t.rowsBeforeCursorFrom(offset) < t.height
}

func (t *jsonTree) renderedRowsFromOffset(offset int) int {
	rows := 0
	for i := offset; i < len(t.visible); i++ {
		rows += t.renderedLineCount(i)
	}
	return rows
}

func (t *jsonTree) scrollbarMarks() []string {
	marks := make([]string, max(0, t.height))
	for i := range marks {
		marks[i] = " "
	}
	if t.height <= 0 || len(t.visible) == 0 {
		return marks
	}
	total := t.renderedRowsFromOffset(0)
	if total <= t.height {
		return marks
	}

	for i := range marks {
		marks[i] = styleScrollTrack.Render("│")
	}
	thumbSize := max(1, t.height*t.height/total)
	if thumbSize > t.height {
		thumbSize = t.height
	}
	scrollable := max(1, total-t.height)
	trackScrollable := max(1, t.height-thumbSize)
	thumbStart := t.renderedRowsBeforeOffset() * trackScrollable / scrollable
	for i := thumbStart; i < thumbStart+thumbSize && i < len(marks); i++ {
		marks[i] = styleScrollThumb.Render("█")
	}
	return marks
}

func (t *jsonTree) renderedRowsBeforeOffset() int {
	rows := 0
	for i := 0; i < t.offset && i < len(t.visible); i++ {
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
	rootIndex := t.cursorRoot()
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
	if !v && rootIndex < len(t.roots) {
		for i, vl := range t.visible {
			if !vl.isSep && vl.node == t.roots[rootIndex] {
				t.cursor = i
				t.offset = 0
				t.clampOffset()
				return
			}
		}
	}
}

func (t *jsonTree) currentPath() string {
	if len(t.visible) == 0 || t.cursor < 0 || t.cursor >= len(t.visible) {
		return ""
	}
	current := t.visible[t.cursor]
	if current.isSep || current.node == nil {
		return ""
	}

	neededDepth := current.depth
	nodes := make([]*jsonNode, neededDepth+1)
	for i := t.cursor; i >= 0 && neededDepth >= 0; i-- {
		vl := t.visible[i]
		if vl.isSep || vl.node == nil || vl.depth != neededDepth {
			continue
		}
		nodes[neededDepth] = vl.node
		neededDepth--
	}
	if len(nodes) == 0 || nodes[0] == nil {
		return ""
	}

	path := nodes[0].key
	for i := 1; i < len(nodes); i++ {
		if nodes[i] == nil || nodes[i].key == "" {
			continue
		}
		path += jsonPathSegment(nodes[i-1], nodes[i].key)
	}
	return path
}

func jsonPathSegment(parent *jsonNode, key string) string {
	if parent != nil && parent.kind == kindArray {
		return "[" + key + "]"
	}
	if isDotPathKey(key) {
		return "." + key
	}
	return "[" + strconv.Quote(key) + "]"
}

func isDotPathKey(s string) bool {
	if s == "" {
		return false
	}
	for i, r := range s {
		switch {
		case r == '_':
		case r >= 'a' && r <= 'z':
		case r >= 'A' && r <= 'Z':
		case i > 0 && r >= '0' && r <= '9':
		default:
			return false
		}
	}
	return true
}
