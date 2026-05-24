package tui

import (
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

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

// Tree styles (styleKey, styleCursor, color* …) live in styles.go.

// jsonNode is one entry in the collapsible tree.
type jsonNode struct {
	key         string
	value       string // rendered value, may include OSC 8 hyperlink sequences
	plainValue  string // value without hyperlink, used on cursor line
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

func bodyRoot(label string, b []byte) *jsonNode {
	if len(b) == 0 {
		return &jsonNode{
			key:      label,
			kind:     kindObject,
			expanded: true,
			children: []*jsonNode{{kind: kindScalar, value: "(empty)", plainValue: "(empty)", scalarColor: colorNull}},
		}
	}
	root := buildNode(label, decodeBody(b))
	root.expanded = true
	return root
}

func decodeBody(b []byte) any {
	var v any
	if err := json.Unmarshal(b, &v); err == nil {
		return v
	}
	if vals, err := url.ParseQuery(string(b)); err == nil && len(vals) > 0 {
		return formToNested(vals)
	}
	return string(b)
}

func formToNested(vals url.Values) map[string]any {
	root := map[string]any{}
	for key, vs := range vals {
		segs := splitFormKey(key)
		cur := root
		for i, s := range segs {
			if i == len(segs)-1 {
				cur[s] = formValue(vs)
				continue
			}
			next, ok := cur[s].(map[string]any)
			if !ok {
				next = map[string]any{}
				cur[s] = next
			}
			cur = next
		}
	}
	return root
}

func splitFormKey(k string) []string {
	i := strings.IndexByte(k, '[')
	if i < 0 {
		return []string{k}
	}
	segs := []string{k[:i]}
	rest := k[i:]
	for len(rest) > 0 && rest[0] == '[' {
		j := strings.IndexByte(rest, ']')
		if j < 0 {
			break
		}
		segs = append(segs, rest[1:j])
		rest = rest[j+1:]
	}
	return segs
}

func formValue(vs []string) any {
	if len(vs) == 1 {
		return vs[0]
	}
	out := make([]any, len(vs))
	for i, s := range vs {
		out[i] = s
	}
	return out
}

func buildNode(key string, v any) *jsonNode {
	switch vv := v.(type) {
	case map[string]any:
		n := &jsonNode{key: key, kind: kindObject, expanded: true}
		for _, k := range sortedKeys(vv) {
			n.children = append(n.children, buildNode(k, vv[k]))
		}
		return n
	case []any:
		n := &jsonNode{key: key, kind: kindArray, expanded: true}
		for i, e := range vv {
			n.children = append(n.children, buildNode(strconv.Itoa(i), e))
		}
		return n
	default:
		plain, sfx, color, dim := renderScalar(v)
		n := &jsonNode{
			key:         key,
			kind:        kindScalar,
			value:       plain,
			plainValue:  plain,
			suffix:      sfx,
			scalarColor: color,
			dim:         dim,
		}
		// Wrap recognised Stripe IDs in a terminal OSC 8 hyperlink.
		if s, ok := v.(string); ok {
			if u := stripeIDURL(s); u != "" {
				n.value = hyperlink(u, plain)
			}
		}
		return n
	}
}

func renderScalar(v any) (text, suffix string, color lipgloss.TerminalColor, dim bool) {
	switch x := v.(type) {
	case string:
		text = strconv.Quote(x)
		// Annotate numeric strings that look like Unix timestamps.
		if ts, err := strconv.ParseFloat(x, 64); err == nil && isUnixTS(ts) {
			suffix = " (" + formatUnix(ts) + ")"
		}
		return text, suffix, colorString, x == ""
	case float64:
		text = strconv.FormatFloat(x, 'f', -1, 64)
		if isUnixTS(x) {
			suffix = " (" + formatUnix(x) + ")"
		}
		return text, suffix, colorNumber, false
	case bool:
		return strconv.FormatBool(x), "", colorBool, false
	case nil:
		return "null", "", colorNull, true
	default:
		return fmt.Sprintf("%v", x), "", colorString, false
	}
}

const (
	minUnixTS = 1_000_000_000.0
	maxUnixTS = 2_000_000_000.0
)

func isUnixTS(f float64) bool {
	return f >= minUnixTS && f <= maxUnixTS && float64(int64(f)) == f
}

func formatUnix(ts float64) string {
	return time.Unix(int64(ts), 0).Format("2006-01-02 15:04:05 MST")
}

// stripeIDURL maps well-known Stripe ID prefixes to their Dashboard URLs.
// Longer prefixes must come first (sub_sched_ before sub_).
func stripeIDURL(id string) string {
	const base = "https://dashboard.stripe.com/test/"
	for _, e := range []struct{ prefix, path string }{
		{"sub_sched_", "subscription_schedules/"},
		{"cus_", "customers/"},
		{"pm_", "payment_methods/"},
		{"pi_", "payment_intents/"},
		{"price_", "prices/"},
		{"prod_", "products/"},
		{"sub_", "subscriptions/"},
		{"in_", "invoices/"},
		{"ch_", "charges/"},
		{"re_", "refunds/"},
		{"acct_", "connect/accounts/"},
		{"txn_", "balance/history/"},
	} {
		if strings.HasPrefix(id, e.prefix) {
			return base + e.path + id
		}
	}
	return ""
}

// hyperlink wraps text in an OSC 8 terminal hyperlink (iTerm2 / WezTerm / Kitty).
func hyperlink(u, text string) string {
	return "\x1b]8;;" + u + "\x07" + text + "\x1b]8;;\x07"
}

func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

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
	if t.cursor < t.offset {
		t.offset = t.cursor
	}
	if t.height > 0 && t.cursor >= t.offset+t.height {
		t.offset = t.cursor - t.height + 1
	}
	if t.offset < 0 {
		t.offset = 0
	}
}

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

func (t *jsonTree) View() string {
	if t.height <= 0 {
		return ""
	}
	var b strings.Builder
	end := t.offset + t.height
	if end > len(t.visible) {
		end = len(t.visible)
	}
	rendered := 0
	for i := t.offset; i < end; i++ {
		b.WriteString(t.renderLine(t.visible[i], i == t.cursor && t.focused))
		b.WriteByte('\n')
		rendered++
	}
	for ; rendered < t.height; rendered++ {
		b.WriteByte('\n')
	}
	return b.String()
}

func (t *jsonTree) renderLine(vl *visibleLine, isCursor bool) string {
	if vl.isSep {
		return ""
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
			return styleCursor.Width(t.width).Render(marker + label)
		}
		head := styleSectionHeader.Render(marker + label)
		fill := t.width - lipgloss.Width(head) - 1
		if fill < 0 {
			fill = 0
		}
		return head + " " + lipgloss.NewStyle().Foreground(colBorder).Render(strings.Repeat("─", fill))
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
		var rest string
		switch {
		case n.kind == kindScalar:
			v := n.plainValue + n.suffix
			if n.key != "" {
				rest = ": " + v
			} else {
				rest = v
			}
		default:
			rest = collapsedSuffix()
		}
		return styleCursor.Width(t.width).Render(indent + marker + n.key + rest)
	}

	var b strings.Builder
	b.WriteString(indent)
	b.WriteString(styleMarker.Render(marker))
	keyStyle := styleKey
	if n.dim {
		keyStyle = styleDim
	}
	// Emphasize keys that actually match the filter (vs. path ancestors kept
	// only to reach a match).
	if t.filterOn && t.filter != "" && n.key != "" && fuzzyMatch(n.key, t.filter) {
		keyStyle = styleMatch
	}
	b.WriteString(keyStyle.Render(n.key))
	switch {
	case n.kind == kindScalar:
		if n.key != "" {
			b.WriteString(styleDim.Render(": "))
		}
		b.WriteString(lipgloss.NewStyle().Foreground(n.scalarColor).Render(n.value))
		if n.suffix != "" {
			b.WriteString(styleDim.Render(n.suffix))
		}
	case !n.expanded:
		b.WriteString(styleDim.Render(collapsedSuffix()))
	}
	return lipgloss.NewStyle().MaxWidth(t.width).Render(b.String())
}
