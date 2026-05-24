package tui

import (
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

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
	case "ctrl+g":
		return tea.KeyMsg{Type: tea.KeyCtrlG}
	case "ctrl+d":
		return tea.KeyMsg{Type: tea.KeyCtrlD}
	case "ctrl+f":
		return tea.KeyMsg{Type: tea.KeyCtrlF}
	case "ctrl+u":
		return tea.KeyMsg{Type: tea.KeyCtrlU}
	case "ctrl+b":
		return tea.KeyMsg{Type: tea.KeyCtrlB}
	case "pgdown":
		return tea.KeyMsg{Type: tea.KeyPgDown}
	case "pgup":
		return tea.KeyMsg{Type: tea.KeyPgUp}
	case "up":
		return tea.KeyMsg{Type: tea.KeyUp}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
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
	if m.focused != focusList {
		t.Fatalf("expected initial focus list, got %q", m.focused)
	}
	m = drive(m, key("tab"))
	if m.focused != focusDetail || !m.tree.focused {
		t.Errorf("tab did not move focus to detail (focused=%q tree.focused=%v)", m.focused, m.tree.focused)
	}
	m = drive(m, key("tab"))
	if m.focused != focusList {
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

func TestShortcutOverlayShowsUnifiedSections(t *testing.T) {
	m := drive(New(), tea.WindowSizeMsg{Width: 100, Height: 30}, key("?"))
	if !m.shortcuts {
		t.Fatal("? did not open shortcuts")
	}
	view := m.View()
	for _, want := range []string{"SHORTCUTS", "Global", "Calls", "Inspector", "Groups", "ctrl+b/f", "ctrl+u/d"} {
		if !strings.Contains(view, want) {
			t.Fatalf("unified shortcuts overlay missing %q:\n%s", want, view)
		}
	}

	before := m.shortcutsOverlay()
	m = drive(m, key("tab"))
	if got := m.shortcutsOverlay(); got != before {
		t.Fatalf("shortcuts overlay changed across panes")
	}

	m = drive(m, key("?"))
	if m.shortcuts {
		t.Fatal("? did not close shortcuts")
	}
}

func TestEnterInspectsSelectedRequestAndEscReturnsToList(t *testing.T) {
	m := drive(New(),
		tea.WindowSizeMsg{Width: 100, Height: 30},
		NewCallMsg(sampleCall()),
	)
	m = drive(m, key("enter"))
	if m.focused != focusDetail || !m.tree.focused {
		t.Fatalf("enter did not inspect request (focused=%q tree.focused=%v)", m.focused, m.tree.focused)
	}
	if !m.hasSel {
		t.Fatal("enter lost selected request")
	}

	m = drive(m, key("esc"))
	if m.focused != focusList || m.tree.focused {
		t.Fatalf("esc did not return to list (focused=%q tree.focused=%v)", m.focused, m.tree.focused)
	}
	if !m.hasSel {
		t.Fatal("esc lost selected request")
	}
}

func TestInspectorHeaderWrapsRequestURLAndGroupName(t *testing.T) {
	longPath := "/v1/invoices?customer=cus_UZpoNO8y6Xj9Sq&subscription=sub_1Tag8rB3ZHLBhbGBjTeVAroS&expand[]=data.customer"
	longGroup := "Group invoices subscription replay with unusually long descriptive name"
	c := sampleCall()
	c.Method = "GET"
	c.Path = longPath
	c.RequestURI = longPath
	c.Group = &proxy.Group{
		ID:       "group-long",
		Name:     longGroup,
		Color:    "Teal",
		LightHex: "#0f766e",
		DarkHex:  "#5eead4",
	}

	m := drive(New(),
		tea.WindowSizeMsg{Width: 84, Height: 32},
		NewCallMsg(c),
	)
	view := m.View()
	for _, want := range []string{
		"expand[]=data.customer",
		"invoices subscription replay",
		"descriptive",
		"name",
		"REQUEST",
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("wrapped inspector header missing %q:\n%s", want, view)
		}
	}
	if got := len(m.detailHeaderLines(m.geometry().rightCW)); got < 4 {
		t.Fatalf("header used %d lines, want wrapped URL and group", got)
	}
	if m.tree.height != m.geometry().treeH {
		t.Fatalf("tree height = %d, want geometry treeH %d", m.tree.height, m.geometry().treeH)
	}
	if !strings.Contains(view, "Group invoices") || strings.Contains(view, "group Group") {
		t.Fatalf("inspector header did not render the group name directly:\n%s", view)
	}
}

func TestInspectorOmitsSubheadRuleWhenKeyFilterInactive(t *testing.T) {
	m := drive(New(),
		tea.WindowSizeMsg{Width: 100, Height: 30},
		NewCallMsg(sampleCall()),
	)
	view := m.View()
	g := m.geometry()
	wantTreeH := g.bodyN - len(m.detailHeaderLines(g.rightCW)) - 1
	if m.tree.height != wantTreeH {
		t.Fatalf("tree height = %d, want %d without inactive divider:\n%s", m.tree.height, wantTreeH, view)
	}

	m = drive(m, key("enter"), key("/"))
	view = m.View()
	if !strings.Contains(view, "filter request keys") {
		t.Fatalf("active key filter bar missing:\n%s", view)
	}
}

func TestNewCallsFollowTopSelection(t *testing.T) {
	first := sampleCall()
	first.Path = "/v1/first"
	first.RequestURI = first.Path
	second := sampleCall()
	second.Path = "/v1/second"
	second.RequestURI = second.Path

	m := drive(New(),
		tea.WindowSizeMsg{Width: 100, Height: 30},
		NewCallMsg(first),
		NewCallMsg(second),
	)
	selected, ok := m.list.SelectedItem().(callItem)
	if !ok {
		t.Fatal("no selected call")
	}
	if selected.call.Path != "/v1/second" || m.list.Index() != 0 {
		t.Fatalf("selected %q at index %d, want newest at top", selected.call.Path, m.list.Index())
	}
	if m.selected.Path != "/v1/second" {
		t.Fatalf("inspector selected %q, want newest call", m.selected.Path)
	}
}

func TestNewCallsPreserveManualOlderSelection(t *testing.T) {
	first := sampleCall()
	first.Path = "/v1/first"
	first.RequestURI = first.Path
	second := sampleCall()
	second.Path = "/v1/second"
	second.RequestURI = second.Path
	third := sampleCall()
	third.Path = "/v1/third"
	third.RequestURI = third.Path

	m := drive(New(),
		tea.WindowSizeMsg{Width: 100, Height: 30},
		NewCallMsg(first),
		NewCallMsg(second),
		key("down"),
		NewCallMsg(third),
	)
	selected, ok := m.list.SelectedItem().(callItem)
	if !ok {
		t.Fatal("no selected call")
	}
	if selected.call.Path != "/v1/first" {
		t.Fatalf("selected %q, want manually selected older call", selected.call.Path)
	}
}

func TestCallListUsesInspectorNavigationKeys(t *testing.T) {
	m := drive(NewWithMaxCalls(30), tea.WindowSizeMsg{Width: 100, Height: 30})
	for i := range 20 {
		c := sampleCall()
		c.Path = fmt.Sprintf("/v1/item_%02d", i)
		c.RequestURI = c.Path
		m = drive(m, NewCallMsg(c))
	}
	full := m.listFullPageStep()
	half := m.listHalfPageStep()

	m = drive(m, key("pgdown"))
	if got := m.list.Index(); got != full {
		t.Fatalf("pgdown selected index %d, want %d", got, full)
	}
	m = drive(m, key("ctrl+u"))
	if got, want := m.list.Index(), full-half; got != want {
		t.Fatalf("ctrl+u selected index %d, want %d", got, want)
	}
	m = drive(m, key("b"))
	if got, want := m.list.Index(), len(m.list.Items())-1; got != want {
		t.Fatalf("b selected index %d, want bottom %d", got, want)
	}
	m = drive(m, key("t"))
	if got := m.list.Index(); got != 0 {
		t.Fatalf("t selected index %d, want top", got)
	}
	m = drive(m, key("ctrl+f"))
	if got := m.list.Index(); got != full {
		t.Fatalf("ctrl+f selected index %d, want %d", got, full)
	}
	m = drive(m, key("ctrl+b"))
	if got := m.list.Index(); got != 0 {
		t.Fatalf("ctrl+b selected index %d, want top", got)
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
	if m.tree.filterOn || m.focused != focusDetail {
		t.Fatalf("esc did not clear filter cleanly (on=%v focused=%q)", m.tree.filterOn, m.focused)
	}
	if !strings.Contains(m.View(), "beta") {
		t.Fatalf("clearing the filter did not restore hidden keys:\n%s", m.View())
	}

	// With the cursor moved into the RESPONSE section, the filter scopes there.
	m = drive(m, key("b"), key("/"))
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

func TestInspectorWrapsLongScalarValues(t *testing.T) {
	longURL := "https://dashboard.stripe.com/acct_123/test/workbench/logs?object=req_ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	tree := jsonTree{width: 32, height: 12, focused: true}
	tree.setCall(proxy.Call{
		ReqBody:  []byte("{}"),
		RespBody: []byte(`{"request_log_url":` + strconv.Quote(longURL) + `}`),
	})
	tree.cursor = 3 // response.request_log_url
	tree.clampOffset()

	view := tree.View()
	lines := strings.Split(strings.TrimRight(view, "\n"), "\n")
	if len(lines) < 2 {
		t.Fatalf("expected wrapped output, got:\n%s", view)
	}
	for _, line := range lines {
		if got := lipgloss.Width(line); got > tree.width {
			t.Fatalf("line width = %d, want <= %d\nline: %q\nview:\n%s", got, tree.width, line, view)
		}
	}
	if !strings.Contains(view, "request_log_url") || !strings.Contains(view, "req_ABC") || !strings.Contains(view, "STUVW") {
		t.Fatalf("wrapped view lost key/value content:\n%s", view)
	}
	if !strings.Contains(view, "\x1b]8;;"+longURL+"\x07") {
		t.Fatalf("wrapped URL was not hyperlinked:\n%s", view)
	}
}

func TestInspectorKeepsCursorVisibleBelowWrappedScalar(t *testing.T) {
	tree := jsonTree{width: 28, height: 5, focused: true}
	tree.setCall(proxy.Call{
		ReqBody:  []byte("{}"),
		RespBody: []byte(`{"long":"` + strings.Repeat("x", 160) + `","zz_after":true}`),
	})
	tree.cursor = tree.skipSepBackward(len(tree.visible) - 1)
	tree.clampOffset()

	view := tree.View()
	if !strings.Contains(view, "zz_after") {
		t.Fatalf("cursor line below wrapped scalar is not visible:\n%s", view)
	}
}

func TestInspectorPageKeysUseFullAndHalfSteps(t *testing.T) {
	body := "{"
	for i := range 30 {
		if i > 0 {
			body += ","
		}
		body += fmt.Sprintf("%q:%d", fmt.Sprintf("k%02d", i), i)
	}
	body += "}"

	base := jsonTree{width: 40, height: 10, focused: true}
	base.setCall(proxy.Call{ReqBody: []byte("{}"), RespBody: []byte(body)})
	base.cursor = 3 // response root

	full := base
	full.Update(key("pgdown"))
	if got, want := full.cursor-base.cursor, 9; got != want {
		t.Fatalf("pgdown moved %d nodes, want %d", got, want)
	}

	ctrlF := base
	ctrlF.Update(key("ctrl+f"))
	if got, want := ctrlF.cursor-base.cursor, 9; got != want {
		t.Fatalf("ctrl+f moved %d nodes, want %d", got, want)
	}

	half := base
	half.Update(key("ctrl+d"))
	if got, want := half.cursor-base.cursor, 5; got != want {
		t.Fatalf("ctrl+d moved %d nodes, want %d", got, want)
	}
}

func TestInspectorTopBottomKeysAvoidGroupToggle(t *testing.T) {
	tree := jsonTree{width: 40, height: 10, focused: true}
	tree.setCall(proxy.Call{
		ReqBody:  []byte("{}"),
		RespBody: []byte(`{"alpha":1,"bravo":2,"charlie":3}`),
	})
	tree.cursor = 3
	tree.Update(key("g"))
	if tree.cursor != 3 {
		t.Fatalf("g moved inspector cursor to %d, want unchanged", tree.cursor)
	}
	tree.Update(key("b"))
	if tree.cursor != len(tree.visible)-1 {
		t.Fatalf("b moved cursor to %d, want bottom %d", tree.cursor, len(tree.visible)-1)
	}
	tree.Update(key("t"))
	if tree.cursor != 0 {
		t.Fatalf("t moved cursor to %d, want top", tree.cursor)
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

func TestRequestGroupingStartsGroupAndAssignsNewCalls(t *testing.T) {
	m := drive(New(),
		tea.WindowSizeMsg{Width: 120, Height: 36},
		key("ctrl+g"),
	)
	if !m.groupsVisible {
		t.Fatal("starting a group did not show the group navigator")
	}
	active := m.groupMgr.Current()
	if active == nil || active.Name != "Group Teal" {
		t.Fatalf("active group = %#v, want Group Teal", active)
	}

	m = drive(m, NewCallMsg(sampleCall()))
	if got := m.allCalls[0].call.Group; got == nil || got.ID != active.ID {
		t.Fatalf("new call group = %#v, want active group %q", got, active.ID)
	}
	view := m.View()
	for _, want := range []string{"GROUPS", "Group Teal", "1 in Group Teal", "█", "▌"} {
		if !strings.Contains(view, want) {
			t.Fatalf("grouped view missing %q\n%s", want, view)
		}
	}
	if strings.Contains(view, "/v1/customers 200  Group Teal") {
		t.Fatalf("request row should not repeat the group name:\n%s", view)
	}
}

func TestGroupNavigatorFiltersCallsByGroup(t *testing.T) {
	groups := NewGroupManager(nil)
	first := groups.Start()
	second := groups.Start()
	second.StartedAt = first.StartedAt.Add(time.Second)

	firstCall := sampleCall()
	firstCall.Path = "/v1/first"
	firstCall.RequestURI = "/v1/first"
	firstCall.Group = &first
	secondCall := sampleCall()
	secondCall.Path = "/v1/second"
	secondCall.RequestURI = "/v1/second"
	secondCall.Group = &second

	m := drive(NewWithCalls(10, []proxy.Call{firstCall, secondCall}),
		tea.WindowSizeMsg{Width: 120, Height: 40},
		key("g"),
		key("tab"),  // focus group navigator
		key("down"), // newest group: second
	)
	if m.focused != focusGroups {
		t.Fatalf("focused = %q, want groups", m.focused)
	}
	view := m.View()
	if !strings.Contains(view, "/v1/second") || strings.Contains(view, "/v1/first") {
		t.Fatalf("group filter did not isolate second group:\n%s", view)
	}

	m = drive(m, key("down")) // next group: first
	view = m.View()
	if !strings.Contains(view, "/v1/first") || strings.Contains(view, "/v1/second") {
		t.Fatalf("group navigation did not move to first group:\n%s", view)
	}
}

func TestGroupManagerSkipsSavedNames(t *testing.T) {
	saved := make([]proxy.Call, 0, 4)
	for i, name := range []string{"Group Teal", "Group Amber", "Group Emerald", "Group Blue"} {
		c := sampleCall()
		c.Group = &proxy.Group{
			ID:        fmt.Sprintf("saved-%d", i),
			Name:      name,
			StartedAt: time.Now().Add(time.Duration(i) * time.Second),
		}
		saved = append(saved, c)
	}

	group := NewGroupManager(saved).Start()
	if group.Name == "Group Emerald" {
		t.Fatalf("new group repeated saved name %q", group.Name)
	}
	if group.Name != "Group Fuchsia" {
		t.Fatalf("new group name = %q, want Group Fuchsia", group.Name)
	}
}

func TestGroupPaneResizesAndScrollsCallList(t *testing.T) {
	m := drive(NewWithMaxCalls(30), tea.WindowSizeMsg{Width: 120, Height: 36})
	for i := range 20 {
		c := sampleCall()
		c.Path = fmt.Sprintf("/v1/item_%02d", i)
		c.RequestURI = c.Path
		m = drive(m, NewCallMsg(c))
	}

	fullHeight := m.list.Height()
	m = drive(m, key("g"))
	groupHeight := m.list.Height()
	if groupHeight != m.geometry().listH {
		t.Fatalf("list height = %d, want geometry listH %d", groupHeight, m.geometry().listH)
	}
	if groupHeight >= fullHeight {
		t.Fatalf("group pane did not reduce list height: before=%d after=%d", fullHeight, groupHeight)
	}

	for range 14 {
		m = drive(m, key("down"))
	}
	selected, ok := m.list.SelectedItem().(callItem)
	if !ok {
		t.Fatal("no selected call")
	}
	if !strings.Contains(m.View(), selected.call.Path) {
		t.Fatalf("selected call is not visible after scrolling with group pane open: %s\n%s", selected.call.Path, m.View())
	}
}
