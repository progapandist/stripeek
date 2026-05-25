package tui

import "strings"

// noMatchNode is the placeholder shown when a section has no matching keys.
var noMatchNode = &jsonNode{kind: kindScalar, value: "(no matches)", plainValue: "(no matches)", scalarColor: colFaint, dim: true}

// fuzzyMatch reports whether query appears contiguously in key
// (case-insensitive). An empty query matches everything.
func fuzzyMatch(key, query string) bool {
	if query == "" {
		return true
	}
	k := strings.ToLower(key)
	q := strings.ToLower(query)
	return strings.Contains(k, q)
}

// walkFilteredRoot renders a section the same way as walkNode (honoring every
// node's expanded flag, so fold / toggle / expand-all all work) but hides keys
// that neither match the key filter nor lead to a match. A matched container
// reveals all of its descendants (subject to their own fold state).
func (t *jsonTree) walkFilteredRoot(root *jsonNode) {
	t.matchCache = map[*jsonNode]bool{}
	t.computeMatch(root)

	t.visible = append(t.visible, &visibleLine{node: root, depth: 0})
	if !root.expanded {
		return
	}
	kept := 0
	for _, c := range root.children {
		if t.matchCache[c] {
			t.walkFilteredNode(c, 1, fuzzyMatch(c.key, t.filter))
			kept++
		}
	}
	if kept == 0 {
		t.visible = append(t.visible, &visibleLine{node: noMatchNode, depth: 1})
	}
}

func (t *jsonTree) walkFilteredNode(n *jsonNode, depth int, forced bool) {
	t.visible = append(t.visible, &visibleLine{node: n, depth: depth})
	if !n.expanded {
		return
	}
	for _, c := range n.children {
		if forced || t.matchCache[c] {
			t.walkFilteredNode(c, depth+1, forced || fuzzyMatch(c.key, t.filter))
		}
	}
}

// computeMatch fills matchCache with whether each node, or any descendant,
// matches the current filter (independent of fold state).
func (t *jsonTree) computeMatch(n *jsonNode) bool {
	m := fuzzyMatch(n.key, t.filter)
	for _, c := range n.children {
		if t.computeMatch(c) {
			m = true
		}
	}
	t.matchCache[n] = m
	return m
}

// cursorRoot returns the index into roots of the section the cursor is in.
func (t *jsonTree) cursorRoot() int {
	for i := t.cursor; i >= 0 && i < len(t.visible); i-- {
		vl := t.visible[i]
		if !vl.isSep && vl.depth == 0 {
			for ri, r := range t.roots {
				if r == vl.node {
					return ri
				}
			}
		}
	}
	return 0
}

// firstFilteredLine is the first real key match across the filtered payload, so
// the cursor lands on a useful match as the filter narrows.
func (t *jsonTree) firstFilteredLine() int {
	fallback := -1
	for i, vl := range t.visible {
		if vl.isSep || vl.node == nil || vl.node == noMatchNode {
			continue
		}
		if fallback < 0 {
			fallback = i
		}
		if t.filter != "" && vl.depth > 0 && fuzzyMatch(vl.node.key, t.filter) {
			return i
		}
	}
	if fallback >= 0 {
		return fallback
	}
	return t.skipSepForward(0)
}

func (t *jsonTree) startFilter() {
	t.typing = true
	t.filterOn = true
	t.filter = ""
	t.rebuild()
	t.cursor = t.firstFilteredLine()
	t.clampOffset()
}

func (t *jsonTree) setFilter(q string) {
	t.filter = q
	t.rebuild()
	t.cursor = t.firstFilteredLine()
	t.clampOffset()
}

func (t *jsonTree) clearFilter() {
	selected := t.current()
	t.typing = false
	t.filterOn = false
	t.filter = ""
	t.rebuild()
	if selected != nil && selected != noMatchNode {
		t.selectNode(selected)
	}
}

func (t *jsonTree) selectNode(target *jsonNode) bool {
	for i, vl := range t.visible {
		if !vl.isSep && vl.node == target {
			t.cursor = i
			t.clampOffset()
			return true
		}
	}
	return false
}
