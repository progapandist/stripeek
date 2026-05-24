package tui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/progapandist/stripeek/proxy"
)

func sampleCall() proxy.Call {
	return proxy.Call{
		Time:       time.Now(),
		Method:     "POST",
		Path:       "/v1/customers",
		RequestURI: "/v1/customers",
		ReqBody:    []byte("email=a%40b.com&metadata[source]=x"),
		Status:     200,
		RespBody:   []byte(`{"id":"cus_123","object":"customer","metadata":{"source":"x"}}`),
		Latency:    42 * time.Millisecond,
	}
}

// drive applies a sequence of messages and returns the resulting model.
func drive(m Model, msgs ...tea.Msg) Model {
	var tm tea.Model = m
	for _, msg := range msgs {
		tm, _ = tm.Update(msg)
	}
	return tm.(Model)
}

func key(s string) tea.KeyMsg {
	switch s {
	case "tab":
		return tea.KeyMsg{Type: tea.KeyTab}
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	default:
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
	}
}

func TestRendersBothPanesAndHelp(t *testing.T) {
	m := drive(New(),
		tea.WindowSizeMsg{Width: 100, Height: 30},
		NewCallMsg(sampleCall()),
	)
	view := m.View()

	for _, want := range []string{"CALLS", "Inspector", "REQUEST", "RESPONSE", "cus_123", "enter"} {
		if !strings.Contains(view, want) {
			t.Errorf("view missing %q\n---\n%s", want, view)
		}
	}
}

func TestCollapseAllHidesNestedFields(t *testing.T) {
	m := drive(New(),
		tea.WindowSizeMsg{Width: 100, Height: 30},
		NewCallMsg(sampleCall()),
		key("tab"), // focus the payload pane
	)
	// "x" is nested inside metadata{}; it should be visible when expanded.
	if !strings.Contains(m.View(), `"x"`) {
		t.Fatalf("expected nested field visible before collapse:\n%s", m.View())
	}

	m = drive(m, key("-")) // collapse all — roots stay visible, their children fold
	if strings.Contains(m.View(), `"x"`) {
		t.Errorf("nested field still visible after collapse-all:\n%s", m.View())
	}
}

func TestTabTogglesFocus(t *testing.T) {
	m := drive(New(), tea.WindowSizeMsg{Width: 100, Height: 30})
	if m.focused != "list" {
		t.Fatalf("expected initial focus list, got %q", m.focused)
	}
	m = drive(m, key("tab"))
	if m.focused != "detail" || !m.tree.focused {
		t.Errorf("tab did not move focus to detail (focused=%q tree.focused=%v)", m.focused, m.tree.focused)
	}
	m = drive(m, key("tab"))
	if m.focused != "list" {
		t.Errorf("second tab did not return focus to list, got %q", m.focused)
	}
}

func TestPaneTitleMarksFocusedPane(t *testing.T) {
	m := drive(New(), tea.WindowSizeMsg{Width: 100, Height: 30})
	view := m.View()
	if !strings.Contains(view, "CALLS") {
		t.Fatalf("list pane was not marked active:\n%s", view)
	}
	if strings.Contains(view, "INSPECTOR") {
		t.Fatalf("inspector mode shown while list focused:\n%s", view)
	}

	m = drive(m, key("tab"))
	view = m.View()
	if !strings.Contains(view, "INSPECTOR") {
		t.Fatalf("inspector pane was not marked active:\n%s", view)
	}
	if !strings.Contains(view, "INSPECTOR") {
		t.Fatalf("inspector mode not shown while payload focused:\n%s", view)
	}
}

func TestEnterInspectsSelectedRequestAndEscReturnsToList(t *testing.T) {
	m := drive(New(),
		tea.WindowSizeMsg{Width: 100, Height: 30},
		NewCallMsg(sampleCall()),
	)
	m = drive(m, key("enter"))
	if m.focused != "detail" || !m.tree.focused {
		t.Fatalf("enter did not inspect request (focused=%q tree.focused=%v)", m.focused, m.tree.focused)
	}
	if !m.hasSel {
		t.Fatal("enter lost selected request")
	}

	m = drive(m, key("esc"))
	if m.focused != "list" || m.tree.focused {
		t.Fatalf("esc did not return to list (focused=%q tree.focused=%v)", m.focused, m.tree.focused)
	}
	if !m.hasSel {
		t.Fatal("esc lost selected request")
	}
}

// filterCall has disjoint request/response key names so a section-scoped
// filter's effect can be asserted unambiguously.
func filterCall() proxy.Call {
	return proxy.Call{
		Time:       time.Now(),
		Method:     "POST",
		Path:       "/v1/things",
		RequestURI: "/v1/things",
		ReqBody:    []byte("alpha[nested]=1&beta=2"),
		Status:     200,
		RespBody:   []byte(`{"zulu":1,"yankee":2}`),
	}
}

func TestInspectorKeyFilterScopedToSection(t *testing.T) {
	m := drive(New(),
		tea.WindowSizeMsg{Width: 100, Height: 40},
		NewCallMsg(filterCall()),
		key("enter"), // inspect, focus the tree (cursor in the REQUEST section)
	)

	// "/" starts a filter scoped to the cursor's section (request here).
	m = drive(m, key("/"))
	if !m.tree.typing || !m.tree.filterOn || m.tree.filterRoot != 0 {
		t.Fatalf("/ did not start a request-scoped filter (typing=%v on=%v root=%d)",
			m.tree.typing, m.tree.filterOn, m.tree.filterRoot)
	}

	m = drive(m, key("a"), key("l")) // "al" fuzzy-matches "alpha" only
	view := m.View()
	if !strings.Contains(view, "alpha") || !strings.Contains(view, "nested") {
		t.Fatalf("matched key/subtree hidden by filter:\n%s", view)
	}
	if curKey(m) != "alpha" {
		t.Fatalf("cursor did not land on the match, got %q", curKey(m))
	}
	if strings.Contains(view, "beta") {
		t.Fatalf("non-matching request key not hidden:\n%s", view)
	}
	// The other section is never filtered — it renders in full.
	if !strings.Contains(view, "zulu") || !strings.Contains(view, "yankee") {
		t.Fatalf("inactive (response) section should render fully:\n%s", view)
	}

	// enter applies (stops typing, keeps the filter); esc then clears it.
	m = drive(m, key("enter"))
	if m.tree.typing || !m.tree.filterOn {
		t.Fatalf("enter should apply filter without clearing (typing=%v on=%v)", m.tree.typing, m.tree.filterOn)
	}
	m = drive(m, key("esc"))
	if m.tree.filterOn || m.focused != "detail" {
		t.Fatalf("esc did not clear filter cleanly (on=%v focused=%q)", m.tree.filterOn, m.focused)
	}
	if !strings.Contains(m.View(), "beta") {
		t.Fatalf("clearing the filter did not restore hidden keys:\n%s", m.View())
	}

	// With the cursor moved into the RESPONSE section, the filter scopes there.
	m = drive(m, key("G"), key("/"))
	if m.tree.filterRoot != 1 {
		t.Fatalf("filter not scoped to response section (root=%d)", m.tree.filterRoot)
	}
	m = drive(m, key("z"), key("u")) // "zu" -> zulu (response)
	if curKey(m) != "zulu" {
		t.Fatalf("response filter did not select \"zulu\", got %q", curKey(m))
	}
	if !strings.Contains(m.View(), "alpha") {
		t.Fatalf("request section should stay fully visible during a response filter:\n%s", m.View())
	}
}

func TestInspectorFilterPreservesFolding(t *testing.T) {
	m := drive(New(),
		tea.WindowSizeMsg{Width: 100, Height: 40},
		NewCallMsg(filterCall()),
		key("enter"),
		key("/"), key("a"), key("l"), key("enter"), // apply filter "al", cursor on alpha
	)
	if !m.tree.filterOn || m.tree.typing || curKey(m) != "alpha" {
		t.Fatalf("expected applied filter on alpha (on=%v typing=%v key=%q)", m.tree.filterOn, m.tree.typing, curKey(m))
	}
	if !strings.Contains(m.View(), "nested") {
		t.Fatalf("alpha's subtree should be visible:\n%s", m.View())
	}

	// All the usual fold controls must keep working with a filter applied.
	m = drive(m, key(" ")) // toggle alpha closed
	if strings.Contains(m.View(), "nested") {
		t.Fatalf("space did not collapse under an active filter:\n%s", m.View())
	}
	m = drive(m, key("+")) // expand all
	if !strings.Contains(m.View(), "nested") {
		t.Fatalf("expand-all did not work under an active filter:\n%s", m.View())
	}
	m = drive(m, key("-")) // collapse all
	if strings.Contains(m.View(), "nested") {
		t.Fatalf("collapse-all did not work under an active filter:\n%s", m.View())
	}
}

func curKey(m Model) string {
	if n := m.tree.current(); n != nil {
		return n.key
	}
	return ""
}

func TestCallHistoryIsBoundedToNewestCalls(t *testing.T) {
	m := drive(NewWithMaxCalls(2), tea.WindowSizeMsg{Width: 100, Height: 30})
	for _, path := range []string{"/v1/oldest", "/v1/middle", "/v1/newest"} {
		c := sampleCall()
		c.Path = path
		c.RequestURI = path + "?expand[]=customer"
		m = drive(m, NewCallMsg(c))
	}

	if len(m.allCalls) != 2 {
		t.Fatalf("len(allCalls) = %d, want 2", len(m.allCalls))
	}
	view := m.View()
	if strings.Contains(view, "/v1/oldest") {
		t.Errorf("oldest call still visible:\n%s", view)
	}
	for _, want := range []string{"/v1/newest?expand[]=customer", "/v1/middle?expand[]=customer"} {
		if !strings.Contains(view, want) {
			t.Errorf("view missing %q\n%s", want, view)
		}
	}
}

func TestNewWithCallsLoadsSavedCallsNewestFirst(t *testing.T) {
	oldest := sampleCall()
	oldest.Path = "/v1/oldest"
	oldest.RequestURI = "/v1/oldest"
	middle := sampleCall()
	middle.Path = "/v1/middle"
	middle.RequestURI = "/v1/middle"
	newest := sampleCall()
	newest.Path = "/v1/newest"
	newest.RequestURI = "/v1/newest"

	m := drive(NewWithCalls(2, []proxy.Call{oldest, middle, newest}), tea.WindowSizeMsg{Width: 100, Height: 30})

	if len(m.allCalls) != 2 {
		t.Fatalf("len(allCalls) = %d, want 2", len(m.allCalls))
	}
	view := m.View()
	if strings.Contains(view, "/v1/oldest") {
		t.Errorf("oldest saved call still visible:\n%s", view)
	}
	if !strings.Contains(view, "/v1/newest") || !strings.Contains(view, "/v1/middle") {
		t.Errorf("saved calls missing from view:\n%s", view)
	}
}
