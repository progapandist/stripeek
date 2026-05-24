package tui

import "strings"

// noMatchNode is the placeholder shown when a section has no matching keys.
var noMatchNode = &jsonNode{kind: kindScalar, value: "(no matches)", plainValue: "(no matches)", scalarColor: colFaint, dim: true}

// fuzzyMatch reports whether query's characters appear in key in order
// (case-insensitive subsequence). An empty query matches everything.
func fuzzyMatch(key, query string) bool {
	if query == "" {
		return true
	}
	k := strings.ToLower(key)
	q := strings.ToLower(query)
	qi := 0
	for i := 0; i < len(k) && qi < len(q); i++ {
		if k[i] == q[qi] {
			qi++
		}
	}
	return qi == len(q)
}

// walkFilteredRoot renders a section the same way as walkNode (honoring every
// node's expanded flag, so fold / toggle / expand-all all work) but hides keys
// that neither match the fuzzy filter nor lead to a match. A matched container
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

// firstFilteredLine is the first key line inside the filtered section, so the
// cursor lands on a match as the filter narrows.
func (t *jsonTree) firstFilteredLine() int {
	for i, vl := range t.visible {
		if vl.isSep || vl.node == nil {
			continue
		}
		if vl.depth == 0 && t.filterRoot < len(t.roots) && t.roots[t.filterRoot] == vl.node {
			if i+1 < len(t.visible) && !t.visible[i+1].isSep {
				return i + 1
			}
			return i
		}
	}
	return t.skipSepForward(0)
}

func (t *jsonTree) startFilter() {
	t.filterRoot = t.cursorRoot()
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
	t.typing = false
	t.filterOn = false
	t.filter = ""
	t.rebuild()
}
