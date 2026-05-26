package tui

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// bodyRoot decodes a captured request or response body into a tree section
// rooted at label. An empty body yields a single "(empty)" placeholder.
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

// headerRoot builds a tree section from an HTTP header map. Single-valued
// headers render as scalars, repeated headers as arrays. The whole section is
// flagged header=true so rendering styles it apart from JSON payload fields.
func headerRoot(label string, h http.Header) *jsonNode {
	if len(h) == 0 {
		return &jsonNode{
			key:      label,
			kind:     kindObject,
			expanded: true,
			header:   true,
			children: []*jsonNode{{kind: kindScalar, value: "(none)", plainValue: "(none)", scalarColor: colorNull, dim: true, header: true}},
		}
	}
	m := make(map[string]any, len(h))
	for k, vs := range h {
		m[k] = formValue(vs)
	}
	root := buildNode(label, m)
	root.expanded = true
	markHeader(root)
	return root
}

// markHeader flags a node subtree as header content and recolors its scalar
// values so they read as metadata rather than JSON payload.
func markHeader(n *jsonNode) {
	n.header = true
	if n.kind == kindScalar && !n.dim {
		n.scalarColor = colorHeaderValue
	}
	for _, c := range n.children {
		markHeader(c)
	}
}

// decodeBody parses a body as JSON, falling back to form-encoded values (Stripe
// requests) and finally to the raw string.
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

// formToNested expands bracketed form keys (metadata[source]=x) into nested maps.
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

// buildNode recursively converts a decoded value into a tree node.
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
			if u := scalarURL(s); u != "" {
				n.linkURL = u
				n.value = hyperlink(u, plain)
			}
		}
		return n
	}
}

// renderScalar formats a leaf value, returning its text, an optional dim suffix
// (e.g. a human-readable timestamp), its color, and whether it should be dimmed.
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

func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
