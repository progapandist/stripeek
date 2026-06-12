package tui

import (
	"fmt"
	"net/http"
	"reflect"
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
	case "ctrl+r":
		return tea.KeyMsg{Type: tea.KeyCtrlR}
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

func numberObject(prefix string, n int) string {
	var b strings.Builder
	b.WriteByte('{')
	for i := range n {
		if i > 0 {
			b.WriteByte(',')
		}
		fmt.Fprintf(&b, "%q:%d", fmt.Sprintf("%s%02d", prefix, i), i)
	}
	b.WriteByte('}')
	return b.String()
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

func TestShortcutOverlayIsGlobal(t *testing.T) {
	m := drive(New(), tea.WindowSizeMsg{Width: 100, Height: 30}, key("?"))
	if !m.shortcuts {
		t.Fatal("? did not open shortcuts")
	}
	view := m.View()
	for _, section := range shortcutSections() {
		if !strings.Contains(view, section.title) {
			t.Fatalf("shortcut overlay missing section %q:\n%s", section.title, view)
		}
	}
	for _, want := range []string{"SHORTCUTS", "ctrl+b/f", "ctrl+u/d"} {
		if !strings.Contains(view, want) {
			t.Fatalf("shortcut overlay missing %q:\n%s", want, view)
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

func TestHelpBarReflectsFocusState(t *testing.T) {
	m := drive(New(),
		tea.WindowSizeMsg{Width: 160, Height: 36},
		NewCallMsg(sampleCall()),
	)
	if got := m.helpBar(); !strings.Contains(got, "enter inspect") || !strings.Contains(got, "g groups") {
		t.Fatalf("list help bar = %q, want inspect and groups shortcuts", got)
	}

	m = drive(m, key("enter"))
	if got := m.helpBar(); !strings.Contains(got, "+/- all") || !strings.Contains(got, "esc back") {
		t.Fatalf("inspector help bar = %q, want expand-all and back shortcuts", got)
	}

	m = drive(m, key("/"))
	if got := m.helpBar(); !strings.Contains(got, "type filter keys") || !strings.Contains(got, "esc cancel") {
		t.Fatalf("filter help bar = %q, want typing shortcuts", got)
	}

	m = drive(New(), tea.WindowSizeMsg{Width: 160, Height: 36}, key("ctrl+g"), key("tab"))
	if m.focused != focusGroups {
		t.Fatalf("test setup focused %q, want groups", m.focused)
	}
	if got := m.helpBar(); !strings.Contains(got, "enter calls") || !strings.Contains(got, "ctrl+g new group") {
		t.Fatalf("groups help bar = %q, want group selection shortcuts", got)
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

// filterCall has disjoint request/response key names so whole-payload filtering
// can be asserted unambiguously.
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

func TestInspectorKeyFilterMatchesRequestAndResponse(t *testing.T) {
	m := drive(New(),
		tea.WindowSizeMsg{Width: 100, Height: 40},
		NewCallMsg(filterCall()),
		key("enter"), // inspect, focus the tree (cursor in the REQUEST section)
	)

	m = drive(m, key("/"))
	if !m.tree.typing || !m.tree.filterOn {
		t.Fatalf("/ did not start a key filter (typing=%v on=%v)", m.tree.typing, m.tree.filterOn)
	}

	m = drive(m, key("a"), key("l")) // "al" matches "alpha" only
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
	if strings.Contains(view, "zulu") || strings.Contains(view, "yankee") {
		t.Fatalf("non-matching response keys should also be hidden:\n%s", view)
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
	view = m.View()
	if !strings.Contains(view, "beta") || !strings.Contains(view, "zulu") || !strings.Contains(view, "yankee") {
		t.Fatalf("clearing the filter did not restore hidden keys:\n%s", view)
	}

	// Starting a filter from the request section still searches response keys.
	m = drive(m, key("t"), key("/"), key("z"), key("u")) // "zu" -> zulu (response)
	if curKey(m) != "zulu" {
		t.Fatalf("whole-payload filter did not select response \"zulu\", got %q", curKey(m))
	}
	if strings.Contains(m.View(), "alpha") {
		t.Fatalf("non-matching request keys should be hidden during a response match:\n%s", m.View())
	}
}

func TestInspectorKeyFilterRequiresContiguousMatch(t *testing.T) {
	if !keyMatchesFilter("end_date", "end") {
		t.Fatal("end should match end_date")
	}
	if !keyMatchesFilter("period_end", "end") {
		t.Fatal("end should match period_end")
	}
	if keyMatchesFilter("collection_method", "end") {
		t.Fatal("end should not match collection_method as a non-contiguous subsequence")
	}
}

func TestInspectorClearFilterKeepsSelectedNestedKey(t *testing.T) {
	m := drive(New(),
		tea.WindowSizeMsg{Width: 110, Height: 34},
		NewCallMsg(proxy.Call{
			Time:       time.Now(),
			Method:     "GET",
			Path:       "/v1/subscription_schedules",
			RequestURI: "/v1/subscription_schedules",
			Status:     200,
			RespBody:   []byte(`{"phases":[{"items":[{"price":{"recurring":{"trial_period_days":14}},"end_date":1780265457}]}],"collection_method":"charge_automatically"}`),
		}),
		key("enter"),
		key("b"),
		key("/"), key("t"), key("r"), key("i"), key("a"), key("l"), key("enter"),
	)
	for moves := 0; curKey(m) != "trial_period_days" && moves < 20; moves++ {
		m = drive(m, key("down"))
	}
	if curKey(m) != "trial_period_days" {
		t.Fatalf("filter did not select nested trial_period_days, got %q", curKey(m))
	}
	pathBefore := m.tree.currentPath()

	m = drive(m, key("esc"))
	if m.tree.filterOn || m.tree.typing {
		t.Fatalf("esc did not clear filter (on=%v typing=%v)", m.tree.filterOn, m.tree.typing)
	}
	if got := curKey(m); got != "trial_period_days" {
		t.Fatalf("clearing filter selected %q, want trial_period_days", got)
	}
	if got := m.tree.currentPath(); got != pathBefore {
		t.Fatalf("clearing filter changed path from %q to %q", pathBefore, got)
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

func TestInspectorFilterRelayoutKeepsCursorVisible(t *testing.T) {
	body := numberObject("end_", 40)

	m := drive(New(),
		tea.WindowSizeMsg{Width: 100, Height: 16},
		NewCallMsg(proxy.Call{
			Time:       time.Now(),
			Method:     "GET",
			Path:       "/v1/large",
			RequestURI: "/v1/large",
			Status:     200,
			RespBody:   []byte(body),
		}),
		key("enter"),
		key("b"),
		key("/"), key("e"), key("n"), key("d"), key("enter"),
	)
	if m.tree.height != m.geometry().treeH {
		t.Fatalf("tree height = %d, want relayout height %d", m.tree.height, m.geometry().treeH)
	}

	for range 25 {
		m = drive(m, key("down"))
	}
	current := curKey(m)
	if current == "" || !strings.Contains(m.View(), current) {
		t.Fatalf("selected filtered key %q is not visible after scrolling:\n%s", current, m.View())
	}
}

func TestInspectorShowsCurrentJSONPath(t *testing.T) {
	m := drive(New(),
		tea.WindowSizeMsg{Width: 110, Height: 32},
		NewCallMsg(proxy.Call{
			Time:       time.Now(),
			Method:     "GET",
			Path:       "/v1/path",
			RequestURI: "/v1/path",
			Status:     200,
			RespBody:   []byte(`{"plan":{"metadata":{"nickname":"Pro"}}}`),
		}),
		key("enter"),
		key("b"),
	)
	for moves := 0; curKey(m) != "nickname" && moves < 20; moves++ {
		m = drive(m, key("down"))
	}
	if curKey(m) != "nickname" {
		t.Fatalf("test setup did not reach nickname, got %q", curKey(m))
	}
	view := m.View()
	if !strings.Contains(view, "path response.plan.metadata.nickname") {
		t.Fatalf("inspector did not show current JSON path:\n%s", view)
	}
}

func TestPanesShowScrollIndicators(t *testing.T) {
	body := numberObject("k", 30)

	m := drive(NewWithMaxCalls(40), tea.WindowSizeMsg{Width: 110, Height: 18})
	for i := range 20 {
		c := sampleCall()
		c.Path = fmt.Sprintf("/v1/item_%02d", i)
		c.RequestURI = c.Path
		if i == 19 {
			c.RespBody = []byte(body)
		}
		m = drive(m, NewCallMsg(c))
	}

	if got := m.callProgressLabel(); !strings.Contains(got, "1/20 ↓") {
		t.Fatalf("call progress label = %q, want top-of-list scroll hint", got)
	}
	marks := strings.Join(m.tree.scrollbarMarks(), "")
	if !strings.Contains(marks, "│") || !strings.Contains(marks, "█") {
		t.Fatalf("scrollbar marks = %q, want track and thumb", marks)
	}
}

func TestCollapseAllFillsViewportFromTopWhenTreeShrinks(t *testing.T) {
	body := "{"
	for i := range 30 {
		if i > 0 {
			body += ","
		}
		body += fmt.Sprintf("%q:{%q:%d}", fmt.Sprintf("group_%02d", i), "nested", i)
	}
	body += "}"

	tree := jsonTree{width: 48, height: 12, focused: true}
	tree.setCall(proxy.Call{ReqBody: []byte("{}"), RespBody: []byte(body)})
	tree.cursor = tree.skipSepBackward(len(tree.visible) - 1)
	tree.clampOffset()
	if tree.offset == 0 {
		t.Fatal("test setup did not scroll the tree")
	}

	tree.setAll(false)
	if tree.offset != 0 {
		t.Fatalf("collapse-all left offset at %d; want top-filled viewport", tree.offset)
	}
	view := tree.View()
	if !strings.Contains(view, "REQUEST") || !strings.Contains(view, "RESPONSE") {
		t.Fatalf("collapsed viewport did not show the top-level context:\n%s", view)
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
	body := numberObject("k", 30)

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
	for _, want := range []string{"GROUPS", "Group Teal", "1 in Group Teal"} {
		if !strings.Contains(view, want) {
			t.Fatalf("grouped view missing %q\n%s", want, view)
		}
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

func TestInspectorTogglesHeaderSections(t *testing.T) {
	tree := jsonTree{width: 60, height: 24, focused: true}
	tree.setCall(proxy.Call{
		ReqBody:        []byte(`{"id":"cus_1"}`),
		RespBody:       []byte(`{"object":"customer"}`),
		RequestHeader:  http.Header{"Idempotency-Key": {"abc"}},
		ResponseHeader: http.Header{"Request-Id": {"req_9"}},
	})

	if got := tree.View(); strings.Contains(got, "HEADERS") {
		t.Fatalf("headers should be hidden by default:\n%s", got)
	}

	tree.Update(key("h"))
	view := tree.View()
	for _, want := range []string{"REQUEST HEADERS", "RESPONSE HEADERS", "Idempotency-Key", "Request-Id"} {
		if !strings.Contains(view, want) {
			t.Fatalf("header toggle did not surface %q:\n%s", want, view)
		}
	}

	tree.Update(key("h"))
	if got := tree.View(); strings.Contains(got, "HEADERS") {
		t.Fatalf("second 'h' should hide headers again:\n%s", got)
	}
}

func TestHeaderNodesAreFlaggedForStyling(t *testing.T) {
	root := headerRoot("request headers", http.Header{"Content-Type": {"application/json"}}, linkContext{})
	if !root.header {
		t.Fatal("header root not flagged")
	}
	if len(root.children) != 1 || !root.children[0].header {
		t.Fatalf("header child not flagged: %+v", root.children)
	}
	if got := root.children[0].scalarColor; got != colorHeaderValue {
		t.Fatalf("header value color = %v, want colorHeaderValue", got)
	}
}

func TestCallRowKeepsStatusVisibleForLongPath(t *testing.T) {
	long := proxy.Call{
		Method:     "GET",
		RequestURI: "/v1/invoices?customer=cus_Ua1Uul5z3jvPFQ&subscription=sub_1TarRPB3ZHLBhbGBcJRIeLSg",
		Status:     200,
		Latency:    477 * time.Millisecond,
		Time:       time.Date(2026, 5, 26, 7, 53, 0, 0, time.UTC),
	}
	item := callItem{call: long}
	top, bottom := item.renderRows(48, false, false, false)

	if lipgloss.Width(top) > 48 || lipgloss.Width(bottom) > 48 {
		t.Fatalf("rows exceed width:\n%q\n%q", top, bottom)
	}
	// Status stays on the top row; latency keeps its own metadata row.
	if !strings.Contains(top, "200") {
		t.Fatalf("top row lost status:\n%q", top)
	}
	if !strings.Contains(bottom, "477ms") || strings.Contains(bottom, "200") {
		t.Fatalf("metadata row should hold only time/latency:\n%q", bottom)
	}
	// Middle truncation keeps the leading resource and the trailing query.
	if !strings.Contains(top, "/v1/invoices") || !strings.Contains(top, "…") || !strings.Contains(top, "RIeLSg") {
		t.Fatalf("top row did not middle-truncate the path:\n%q", top)
	}
}

func TestCallRowMarksDirectionByKind(t *testing.T) {
	outbound := callItem{call: proxy.Call{
		Method:     "POST",
		RequestURI: "/v1/subscriptions",
		Status:     200,
		Time:       time.Date(2026, 5, 26, 7, 53, 0, 0, time.UTC),
	}}
	webhook := callItem{call: proxy.Call{
		Method:     "POST",
		RequestURI: "/webhooks",
		Status:     200,
		IsWebhook:  true,
		Time:       time.Date(2026, 5, 26, 7, 53, 0, 0, time.UTC),
	}}

	outTop, _ := outbound.renderRows(48, false, false, false)
	if !strings.Contains(outTop, glyphOutbound) || strings.Contains(outTop, glyphInbound) {
		t.Fatalf("outbound row should carry the ▶ glyph only:\n%q", outTop)
	}
	whTop, _ := webhook.renderRows(48, false, false, false)
	if !strings.Contains(whTop, glyphInbound) || strings.Contains(whTop, glyphOutbound) {
		t.Fatalf("webhook row should carry the ◀ glyph only:\n%q", whTop)
	}
}

func TestWebhookMetaExtractsEventFields(t *testing.T) {
	body := []byte(`{"id":"evt_1","type":"customer.subscription.created","livemode":false,` +
		`"request":{"id":"req_77","idempotency_key":"k"},` +
		`"data":{"object":{"id":"sub_abc123","object":"subscription","customer":"cus_9",` +
		`"test_clock":"clock_5","latest_invoice":"in_2","application":null}}}`)
	got := webhookMeta(body)
	if got.eventType != "customer.subscription.created" || got.eventID != "evt_1" ||
		got.livemode != "test" || got.requestID != "req_77" {
		t.Fatalf("webhookMeta = %+v", got)
	}
	if !reflect.DeepEqual(got.refs, []string{"sub_abc123", "cus_9", "clock_5", "in_2"}) {
		t.Fatalf("refs = %v", got.refs)
	}

	// Legacy request form (bare string) and async events (request null).
	if got := webhookMeta([]byte(`{"type":"t","request":"req_old"}`)); got.requestID != "req_old" {
		t.Fatalf("legacy request = %+v", got)
	}
	if got := webhookMeta([]byte(`{"type":"t","request":null}`)); got.requestID != "" {
		t.Fatalf("null request = %+v", got)
	}

	if got := webhookMeta([]byte(`{"id":"evt_2","type":"charge.succeeded","livemode":true}`)); got.livemode != "live" {
		t.Fatalf("livemode true should map to %q, got %+v", "live", got)
	}
	// Absent livemode stays unknown rather than defaulting to test.
	if got := webhookMeta([]byte(`{"id":"evt_3","type":"charge.succeeded"}`)); got.livemode != "" {
		t.Fatalf("missing livemode should stay empty, got %+v", got)
	}

	// Truncated / non-event bodies yield the zero value so callers fall back.
	for _, bad := range [][]byte{
		[]byte(`{"id":"evt_1","type":"customer.subscriptio`), // truncated JSON
		[]byte(`not json at all`),
		nil,
	} {
		if got := webhookMeta(bad); got.eventType != "" || got.eventID != "" ||
			got.requestID != "" || len(got.refs) != 0 {
			t.Fatalf("webhookMeta(%q) = %+v, want zero value", bad, got)
		}
	}
}

func TestOutboundSeedRefs(t *testing.T) {
	refs := outboundSeedRefs(
		"/v1/test_helpers/test_clocks/clock_77/advance?customer=cus_1&starting_after=cus_1",
		[]byte(`{"id":"clock_77","object":"test_helpers.test_clock","customer":"cus_1","latest_invoice":"in_3"}`))
	if !reflect.DeepEqual(refs, []string{"clock_77", "cus_1", "in_3"}) {
		t.Fatalf("refs = %v", refs)
	}
	// List responses contribute nothing beyond recognised top-level fields,
	// and nested data ids must not leak into the seed.
	refs = outboundSeedRefs("/v1/invoices?status=open",
		[]byte(`{"object":"list","data":[{"id":"in_a","customer":"cus_b"}]}`))
	if len(refs) != 0 {
		t.Fatalf("list response should seed nothing, got %v", refs)
	}
}

func TestWebhookRowUsesEventNameAndFilter(t *testing.T) {
	hook := callItem{
		call:    proxy.Call{IsWebhook: true, Method: "POST", RequestURI: "/webhooks", Status: 200, Time: time.Date(2026, 5, 26, 7, 53, 0, 0, time.UTC)},
		webhook: webhookInfo{eventType: "customer.subscription.created"},
	}
	out := callItem{
		call: proxy.Call{Method: "GET", RequestURI: "/v1/customers", Status: 200, Time: time.Date(2026, 5, 26, 7, 53, 0, 0, time.UTC)},
	}

	// FilterValue: event name for webhooks, path for outbound.
	if got := hook.FilterValue(); got != "customer.subscription.created" {
		t.Fatalf("webhook FilterValue = %q", got)
	}
	if got := out.FilterValue(); got != "/v1/customers" {
		t.Fatalf("outbound FilterValue = %q", got)
	}

	// The row reads by event name and drops the POST method token.
	top, _ := hook.renderRows(60, false, false, false)
	if !strings.Contains(top, "customer.subscription.created") {
		t.Fatalf("webhook row lost event name:\n%q", top)
	}
	if strings.Contains(top, "POST") || strings.Contains(top, "/webhooks") {
		t.Fatalf("webhook row should not show method or path:\n%q", top)
	}

	// A webhook with no parsed event type falls back to the path label.
	bare := callItem{call: proxy.Call{IsWebhook: true, Method: "POST", RequestURI: "/webhooks", Status: 200}}
	if got := bare.FilterValue(); got != "/webhooks" {
		t.Fatalf("bare webhook FilterValue = %q, want path fallback", got)
	}
	bareTop, _ := bare.renderRows(60, false, false, false)
	if !strings.Contains(bareTop, "/webhooks") {
		t.Fatalf("bare webhook row should fall back to path:\n%q", bareTop)
	}
}

func TestInspectorHeaderShowsEventForWebhook(t *testing.T) {
	hook := sampleCall()
	hook.IsWebhook = true
	hook.RequestURI = "/webhooks"
	hook.Path = "/webhooks"
	hook.ReqBody = []byte(`{"id":"evt_42","type":"customer.subscription.created","livemode":false,"data":{"object":{"id":"sub_1"}}}`)

	m := drive(New(), tea.WindowSizeMsg{Width: 100, Height: 30}, NewCallMsg(hook))
	header := strings.Join(m.detailHeaderLines(60), "\n")
	for _, want := range []string{"customer.subscription.created", "evt_42", "200", "TEST"} {
		if !strings.Contains(header, want) {
			t.Fatalf("webhook header missing %q:\n%s", want, header)
		}
	}
	for _, reject := range []string{"POST", "/webhooks"} {
		if strings.Contains(header, reject) {
			t.Fatalf("webhook header should not show %q:\n%s", reject, header)
		}
	}

	// A webhook whose body didn't parse keeps the method/path header.
	bare := sampleCall()
	bare.IsWebhook = true
	bare.RequestURI = "/webhooks"
	bare.Path = "/webhooks"
	bare.ReqBody = []byte(`not json`)
	m = drive(New(), tea.WindowSizeMsg{Width: 100, Height: 30}, NewCallMsg(bare))
	header = strings.Join(m.detailHeaderLines(60), "\n")
	if !strings.Contains(header, "POST") || !strings.Contains(header, "/webhooks") {
		t.Fatalf("unparseable webhook should fall back to method/path header:\n%s", header)
	}

	// Outbound calls are unchanged: method/path title, key-derived badge.
	out := sampleCall()
	out.KeyMode = "live"
	m = drive(New(), tea.WindowSizeMsg{Width: 100, Height: 30}, NewCallMsg(out))
	header = strings.Join(m.detailHeaderLines(60), "\n")
	for _, want := range []string{"POST", "/v1/customers", "LIVE"} {
		if !strings.Contains(header, want) {
			t.Fatalf("outbound header missing %q:\n%s", want, header)
		}
	}
}

func TestTruncateMiddleKeepsEnds(t *testing.T) {
	got := truncateMiddle("/v1/customers/cus_ABCDEFGHIJKLMNOP/sources", 20)
	if lipgloss.Width(got) > 20 {
		t.Fatalf("truncated wider than 20: %q (%d)", got, lipgloss.Width(got))
	}
	if !strings.HasPrefix(got, "/v1/") || !strings.HasSuffix(got, "sources") || !strings.Contains(got, "…") {
		t.Fatalf("middle truncation lost an end: %q", got)
	}
	if short := truncateMiddle("/v1/ok", 20); short != "/v1/ok" {
		t.Fatalf("fitting string should pass through unchanged, got %q", short)
	}
}

func TestCallRowShortPathKeepsStatusOnTop(t *testing.T) {
	item := callItem{call: proxy.Call{
		Method:     "GET",
		RequestURI: "/v1/customers",
		Status:     404,
		Latency:    12 * time.Millisecond,
		Time:       time.Date(2026, 5, 26, 7, 53, 0, 0, time.UTC),
	}}
	top, bottom := item.renderRows(60, false, false, false)
	if !strings.Contains(top, "404") {
		t.Fatalf("short path should keep status on top row:\n%q", top)
	}
	if strings.Contains(bottom, "404") {
		t.Fatalf("status should not also appear on metadata row:\n%q", bottom)
	}
}

func TestMethodStyleSeparatesMutating(t *testing.T) {
	// lipgloss strips color outside a TTY, so compare the configured foreground
	// rather than rendered output.
	safe := styleMethodSafe.GetForeground()
	write := styleMethodWrite.GetForeground()
	if safe == write {
		t.Fatal("safe and mutating methods must use distinct colors")
	}
	for _, m := range []string{"GET", "HEAD", "OPTIONS", "TRACE", "get"} {
		if got := methodStyle(m).GetForeground(); got != safe {
			t.Fatalf("%q classified as mutating, want safe", m)
		}
	}
	for _, m := range []string{"POST", "PUT", "PATCH", "DELETE", "WEIRD"} {
		if got := methodStyle(m).GetForeground(); got != write {
			t.Fatalf("%q classified as safe, want mutating", m)
		}
	}
}

func TestStripeIDURLUsesMode(t *testing.T) {
	cases := []struct {
		name string
		id   string
		ctx  linkContext
		want string
	}{
		{"test", "sub_123", linkContext{mode: "test"}, "https://dashboard.stripe.com/test/subscriptions/sub_123"},
		{"live", "sub_123", linkContext{mode: "live"}, "https://dashboard.stripe.com/subscriptions/sub_123"},
		{"unknown defaults to test", "cus_9", linkContext{}, "https://dashboard.stripe.com/test/customers/cus_9"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := stripeIDURL(tc.id, tc.ctx); got != tc.want {
				t.Fatalf("stripeIDURL = %q, want %q", got, tc.want)
			}
		})
	}
	if got := scalarURL("https://already.url/x", linkContext{mode: "live"}); got != "https://already.url/x" {
		t.Fatalf("scalarURL should pass URLs through, got %q", got)
	}
}

func TestInspectorSurfacesModeBadge(t *testing.T) {
	c := sampleCall()
	c.KeyMode = "live"
	m := drive(New(), tea.WindowSizeMsg{Width: 100, Height: 30}, NewCallMsg(c))
	view := m.View()
	if !strings.Contains(view, "LIVE") {
		t.Fatalf("inspector header missing LIVE badge:\n%s", view)
	}

	// The live customer link in the payload must drop the test segment.
	if !strings.Contains(view, "\x1b]8;;https://dashboard.stripe.com/customers/cus_123") {
		t.Fatalf("payload link did not use live mode:\n%s", view)
	}

	if badge := modeBadge(""); badge != "" {
		t.Fatalf("modeBadge with unknown mode = %q, want empty", badge)
	}
}

func TestInspectorJumpToBottomLargeTreeIsResponsive(t *testing.T) {
	// A big response payload: pre-fix this jump was O(N^2) full re-renders and
	// would hang. Guard that it completes quickly and lands on the last field.
	body := numberObject("field_", 6000)
	tree := jsonTree{width: 60, height: 30, focused: true}
	tree.setCall(proxy.Call{ReqBody: []byte("{}"), RespBody: []byte(body)})

	done := make(chan struct{})
	go func() {
		tree.Update(key("b")) // jump to bottom
		_ = tree.View()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("jump-to-bottom did not complete within 2s (perf regression)")
	}

	wantCursor := tree.skipSepBackward(len(tree.visible) - 1)
	if tree.cursor != wantCursor {
		t.Fatalf("cursor = %d, want last visible row %d", tree.cursor, wantCursor)
	}
	last := tree.visible[tree.cursor].node
	if last == nil || !strings.Contains(tree.View(), last.key) {
		t.Fatalf("last field %q not visible after jump:\n%s", lastKey(last), tree.View())
	}
}

func lastKey(n *jsonNode) string {
	if n == nil {
		return "<nil>"
	}
	return n.key
}

func BenchmarkInspectorJumpToBottom(b *testing.B) {
	body := numberObject("field_", 6000)
	tree := jsonTree{width: 60, height: 30, focused: true}
	tree.setCall(proxy.Call{ReqBody: []byte("{}"), RespBody: []byte(body)})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tree.Update(key("t")) // top
		tree.Update(key("b")) // bottom
	}
}

func TestLineRowsMatchesRenderLines(t *testing.T) {
	longVal := strings.Repeat("x", 300)
	url := "https://dashboard.stripe.com/test/invoices/in_" + strings.Repeat("A", 120)
	body := `{"short":"ok","empty":"","num":2499,"nested":{"a":1,"b":[1,2,3]},` +
		`"long":"` + longVal + `","url":"` + url + `","bool":true,"nil":null}`

	tree := jsonTree{width: 32, height: 40, focused: true}
	tree.setCall(proxy.Call{ReqBody: []byte("{}"), RespBody: []byte(body)})

	for i, vl := range tree.visible {
		want := max(1, len(tree.renderLines(vl, false)))
		if got := tree.lineRows(vl); got != want {
			t.Fatalf("line %d (%+v): lineRows=%d, renderLines=%d", i, vl.node, got, want)
		}
	}
}

func TestInspectorLongValuesStayResponsive(t *testing.T) {
	// Invoice-like payload with many long string values; the old O(L^2) wrap
	// inside a full-tree count made this hang.
	var b strings.Builder
	b.WriteString(`{"data":[`)
	for i := 0; i < 200; i++ {
		if i > 0 {
			b.WriteByte(',')
		}
		fmt.Fprintf(&b, `{"id":"in_%d","url":%q,"description":%q}`,
			i, "https://invoice.stripe.com/i/acct_X/"+strings.Repeat("Z", 200),
			strings.Repeat("long description text ", 30))
	}
	b.WriteString(`]}`)

	tree := jsonTree{width: 50, height: 30, focused: true}

	done := make(chan struct{})
	go func() {
		tree.setCall(proxy.Call{ReqBody: []byte("{}"), RespBody: []byte(b.String())})
		tree.Update(key("b"))
		_ = tree.View()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("long-value payload did not render within 3s (perf regression)")
	}
}

// opCall builds an outbound call for operation tests.
func opCall(method, uri, reqID, respBody string) proxy.Call {
	return proxy.Call{
		Time:            time.Now(),
		Method:          method,
		Path:            uri,
		RequestURI:      uri,
		Status:          200,
		StripeRequestID: reqID,
		RespBody:        []byte(respBody),
		Latency:         5 * time.Millisecond,
	}
}

// opEvent builds a webhook capture whose body carries the given request id
// ("" → null, an async event) and data.object JSON.
func opEvent(eventType, reqID, object string) proxy.Call {
	req := "null"
	if reqID != "" {
		req = fmt.Sprintf(`{"id":%q,"idempotency_key":null}`, reqID)
	}
	body := fmt.Sprintf(`{"id":"evt_x","type":%q,"livemode":false,"request":%s,"data":{"object":%s}}`,
		eventType, req, object)
	return proxy.Call{
		Time:       time.Now(),
		Method:     "POST",
		Path:       "/webhooks",
		RequestURI: "/webhooks",
		Status:     200,
		IsWebhook:  true,
		ReqBody:    []byte(body),
	}
}

// operationScenario mirrors the shape of a real captured test-client run:
// a subscription create with sync events, a polling read, a clock advance
// whose async cascade chains by refs, an orphan pair (trigger not proxied),
// and a ref-less event.
func operationScenario() []proxy.Call {
	return []proxy.Call{
		opCall("POST", "/v1/subscriptions", "req_A", `{"id":"sub_1","customer":"cus_1","latest_invoice":"in_1"}`),
		opEvent("customer.subscription.created", "req_A", `{"id":"sub_1","customer":"cus_1"}`),
		opEvent("invoice.paid", "req_A", `{"id":"in_1","customer":"cus_1","subscription":"sub_1"}`),
		opCall("GET", "/v1/subscriptions/sub_1", "req_B", `{"id":"sub_1","customer":"cus_1"}`),
		opCall("POST", "/v1/test_helpers/test_clocks/clock_1/advance", "req_C", `{"id":"clock_1"}`),
		opEvent("invoice.created", "", `{"id":"in_2","customer":"cus_1","subscription":"sub_1","test_clock":"clock_1"}`),
		opEvent("charge.succeeded", "", `{"id":"ch_1","customer":"cus_1","invoice":"in_2"}`),
		opEvent("product.created", "req_Z", `{"id":"prod_9"}`),
		opEvent("price.created", "req_Z", `{"id":"price_9"}`),
		opEvent("balance.available", "", `{"object":"balance"}`),
	}
}

func TestOperationsLinkByRequestIDAndAdoption(t *testing.T) {
	msgs := []tea.Msg{tea.WindowSizeMsg{Width: 100, Height: 40}}
	for _, c := range operationScenario() {
		msgs = append(msgs, NewCallMsg(c))
	}
	m := drive(New(), msgs...)

	// allCalls is newest-first; index 9 is the subscription POST.
	op := func(i int) uint64 { return m.allCalls[i].opID }
	subOp, advOp := op(9), op(5)
	if subOp == 0 || !m.allCalls[9].isAnchor {
		t.Fatalf("subscription POST should anchor an operation, got op=%d anchor=%v", subOp, m.allCalls[9].isAnchor)
	}
	if op(8) != subOp || op(7) != subOp {
		t.Fatalf("sync events should join by request id: got %d,%d want %d", op(8), op(7), subOp)
	}
	if op(6) != 0 {
		t.Fatalf("GET should never get an operation, got %d", op(6))
	}
	if advOp == 0 || advOp == subOp || !m.allCalls[5].isAnchor {
		t.Fatalf("advance should anchor its own operation, got %d (sub %d)", advOp, subOp)
	}
	if op(4) != advOp {
		t.Fatalf("async invoice.created should adopt into the advance op via clock ref, got %d want %d", op(4), advOp)
	}
	if op(3) != advOp {
		t.Fatalf("charge.succeeded should chain via the accreted invoice ref, got %d want %d", op(3), advOp)
	}
	orphan := op(2)
	if orphan == 0 || orphan == subOp || orphan == advOp || m.allCalls[2].isAnchor {
		t.Fatalf("orphan event should mint an anchor-less op, got %d anchor=%v", orphan, m.allCalls[2].isAnchor)
	}
	if op(1) != orphan {
		t.Fatalf("orphan sibling sharing the request id should join, got %d want %d", op(1), orphan)
	}
	if op(0) != 0 {
		t.Fatalf("event with no request id and no known refs should stay loose, got %d", op(0))
	}

	// History replay (oldest-first) reproduces the same assignment.
	replay := drive(NewWithCalls(50, operationScenario()), tea.WindowSizeMsg{Width: 100, Height: 40})
	for i := range m.allCalls {
		if replay.allCalls[i].opID != m.allCalls[i].opID {
			t.Fatalf("replay diverged at %d: %d vs %d", i, replay.allCalls[i].opID, m.allCalls[i].opID)
		}
	}
}

func TestRelationFilterFocusesOperationAndBypassesGroupFilter(t *testing.T) {
	grp := &proxy.Group{ID: "g1", Name: "manual group", Color: "Teal", LightHex: "#0f766e", DarkHex: "#5eead4"}
	anchor := opCall("POST", "/v1/subscriptions", "req_A", `{"id":"sub_1","customer":"cus_1"}`)
	anchor.Group = grp
	m := drive(New(), tea.WindowSizeMsg{Width: 100, Height: 40},
		NewCallMsg(opCall("POST", "/v1/customers", "req_0", `{"id":"cus_1"}`)),
		NewCallMsg(anchor),
		NewCallMsg(opEvent("customer.subscription.created", "req_A", `{"id":"sub_1"}`)),
		NewCallMsg(opCall("GET", "/v1/invoices", "req_B", `{"object":"list"}`)),
	)

	// Selecting a member shows the hint in the inspector header.
	m = drive(m, key("down")) // newest-first: GET, event, anchor, customers — select the event
	header := strings.Join(m.detailHeaderLines(60), "\n")
	if !strings.Contains(header, "1 related") {
		t.Fatalf("header should hint at the related anchor:\n%s", header)
	}

	// With a group filter hiding everything but the anchor, relation mode
	// still shows every member of the operation.
	m.groupFilterID = "g1"
	m.rebuildList()
	if got := len(m.list.Items()); got != 1 {
		t.Fatalf("group filter precondition: want 1 item, got %d", got)
	}
	m = drive(m, key("r"))
	if m.relationOpID == 0 || m.relationDim {
		t.Fatalf("r should enter relation filter mode, got op=%d dim=%v", m.relationOpID, m.relationDim)
	}
	if got := len(m.list.Items()); got != 2 {
		t.Fatalf("relation mode should bypass the group filter: want anchor+event, got %d", got)
	}
	// Order stays global newest-first: the event on top, the anchor below.
	first := m.list.Items()[0].(callItem)
	last := m.list.Items()[1].(callItem)
	if first.isAnchor || !last.isAnchor {
		t.Fatalf("relation mode should keep newest-first order, got anchor flags %v,%v",
			first.isAnchor, last.isAnchor)
	}
	if last.call.Group == nil {
		t.Fatal("relation mode must not strip manual group decoration")
	}
	if line := m.callCountLine(); !strings.Contains(line, "in operation POST /v1/subscriptions") {
		t.Fatalf("count line should name the operation:\n%q", line)
	}
	// No hint while the mode is active.
	if h := strings.Join(m.detailHeaderLines(60), "\n"); strings.Contains(h, "related") {
		t.Fatalf("hint should be suppressed inside relation mode:\n%s", h)
	}

	// esc exits back to the group-filtered view.
	m = drive(m, key("esc"))
	if m.relationOpID != 0 || len(m.list.Items()) != 1 {
		t.Fatalf("esc should exit relation mode and restore the group filter, got op=%d items=%d",
			m.relationOpID, len(m.list.Items()))
	}

	// ctrl+r dims instead: full timeline stays in the list.
	m = drive(m, key("ctrl+r"))
	if m.relationOpID == 0 || !m.relationDim {
		t.Fatalf("ctrl+r should enter dim mode, got op=%d dim=%v", m.relationOpID, m.relationDim)
	}
	if got := len(m.list.Items()); got != 4 {
		t.Fatalf("dim mode should keep every row visible, got %d", got)
	}
	if line := m.callCountLine(); !strings.Contains(line, "2 related to POST /v1/subscriptions") {
		t.Fatalf("dim count line should report members:\n%q", line)
	}
	m = drive(m, key("esc"))
	if m.relationOpID != 0 {
		t.Fatal("esc should exit dim mode")
	}
}

func TestRelationInertWithoutRelatedTraffic(t *testing.T) {
	m := drive(New(), tea.WindowSizeMsg{Width: 100, Height: 30},
		NewCallMsg(opCall("POST", "/v1/customers", "req_1", `{"id":"cus_1"}`)),
		NewCallMsg(opCall("GET", "/v1/customers/cus_1", "req_2", `{"id":"cus_1"}`)),
	)
	for _, k := range []string{"r", "ctrl+r"} {
		m = drive(m, key(k))
		if m.relationOpID != 0 {
			t.Fatalf("%s should be inert without related traffic", k)
		}
	}
	m = drive(m, key("down"), key("r")) // the mutation: single-member op, still inert
	if m.relationOpID != 0 {
		t.Fatal("single-member operation should not enter relation mode")
	}
	if h := strings.Join(m.detailHeaderLines(60), "\n"); strings.Contains(h, "related") {
		t.Fatalf("no hint expected without webhook traffic:\n%s", h)
	}
}
