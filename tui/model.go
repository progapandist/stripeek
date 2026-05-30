package tui

import (
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/progapandist/stripeek/proxy"
)

const DefaultMaxCalls = 1000

// focusZone identifies which pane currently receives key input.
type focusZone int

const (
	focusList focusZone = iota
	focusGroups
	focusDetail
)

func (f focusZone) String() string {
	switch f {
	case focusList:
		return "list"
	case focusGroups:
		return "groups"
	case focusDetail:
		return "detail"
	default:
		return "unknown"
	}
}

// NewCallMsg carries a newly captured call into the TUI.
type NewCallMsg proxy.Call

// DroppedMsg reports the running total of captures dropped because the TUI
// couldn't keep up with the proxy.
type DroppedMsg int

// Model is the root Bubble Tea model.
type Model struct {
	list          list.Model
	tree          jsonTree
	allCalls      []callItem // all captured calls, newest first
	maxCalls      int
	nextID        uint64
	filter        string // path substring filter
	filtering     bool   // true while typing a filter
	selected      proxy.Call
	hasSel        bool
	loadedID      uint64
	width         int
	height        int
	focused       focusZone
	groupMgr      *GroupManager
	groupsVisible bool
	groupCursor   int
	groupFilterID string
	groups        []groupEntry
	shortcuts     bool
	dropped       int    // captures lost because the TUI fell behind the proxy
	OnClear       func() // called after in-memory history is wiped
}

func New() Model {
	return NewWithMaxCalls(DefaultMaxCalls)
}

func NewWithMaxCalls(maxCalls int) Model {
	return NewWithCalls(maxCalls, nil)
}

func NewWithCalls(maxCalls int, calls []proxy.Call) Model {
	return NewWithGroupManager(maxCalls, calls, NewGroupManager(calls))
}

func NewWithGroupManager(maxCalls int, calls []proxy.Call, groupMgr *GroupManager) Model {
	if groupMgr == nil {
		groupMgr = NewGroupManager(calls)
	}

	l := list.New(nil, callDelegate{}, 0, 0)
	l.SetShowTitle(false)
	l.SetShowStatusBar(false)
	l.SetShowPagination(false)
	l.SetShowHelp(false)
	l.SetFilteringEnabled(false)

	m := Model{
		list:     l,
		maxCalls: maxCalls,
		focused:  focusList,
		groupMgr: groupMgr,
	}
	for _, c := range calls {
		m.prependCall(c)
	}
	m.rebuildList()
	return m
}

func (m Model) Init() tea.Cmd { return nil }

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.layout()

	case tea.KeyMsg:
		cmd, handled, quit := m.handleGlobalKey(msg)
		switch {
		case quit:
			return m, tea.Quit
		case handled:
			return m, cmd
		}
		return m, m.routeKey(msg)

	case NewCallMsg:
		followTop := len(m.list.Items()) == 0 || m.list.Index() == 0
		call := proxy.Call(msg)
		if call.Group == nil {
			if m.groupMgr == nil {
				m.groupMgr = NewGroupManager(nil)
			}
			if group := m.groupMgr.Current(); group != nil && !call.Time.Before(group.StartedAt) {
				call.Group = group
			}
		}
		m.prependCall(call)
		m.rebuildList()
		if followTop && len(m.list.Items()) > 0 {
			m.list.Select(0)
			m.syncTree()
		}
		return m, nil

	case DroppedMsg:
		m.dropped = int(msg)
		return m, nil
	}

	return m, cmd
}

// handleGlobalKey processes shortcuts that apply regardless of the focused pane
// and reports whether it consumed the key (handled) and whether the program
// should quit. The case order is significant: it mirrors the original precedence
// so that, for example, q quits even while the shortcuts overlay is open, and
// tab still switches panes with the overlay up.
func (m *Model) handleGlobalKey(msg tea.KeyMsg) (cmd tea.Cmd, handled, quit bool) {
	switch {
	case matches(msg, keyForceQuit):
		return nil, true, true
	case m.shortcuts && (matches(msg, keyShortcuts) || matches(msg, keyDismiss)):
		m.shortcuts = false
		return nil, true, false
	case matches(msg, keyQuit) && !m.inputActive():
		// q quits unless the user is typing (list filter or inspector search).
		return nil, true, true
	case matches(msg, keyShortcuts) && !m.inputActive():
		m.shortcuts = true
		return nil, true, false
	case matches(msg, keyNextPane) || matches(msg, keyPrevPane):
		// Tab switches panes/sections, cancelling any in-progress text input.
		m.advanceFocus(matches(msg, keyPrevPane))
		m.tree.focused = m.focused == focusDetail
		m.layout()
		return nil, true, false
	case m.shortcuts:
		// Overlay open: swallow every remaining key.
		return nil, true, false
	case matches(msg, keyNewGroup) && !m.inputActive():
		m.startGroup()
		m.tree.focused = m.focused == focusDetail
		return nil, true, false
	case matches(msg, keyClearHistory) && !m.inputActive():
		m.clearHistory()
		return nil, true, false
	case m.focused == focusDetail && matches(msg, keyDismiss) && !m.tree.typing && !m.tree.filterOn:
		// esc in the inspector returns to the list, unless a key filter is active
		// (then esc clears the filter — handled by the tree).
		m.focused = focusList
		m.tree.focused = false
		return nil, true, false
	}
	return nil, false, false
}

// routeKey dispatches a key the global handler didn't consume to the focused pane.
func (m *Model) routeKey(msg tea.KeyMsg) tea.Cmd {
	switch m.focused {
	case focusList:
		return m.updateList(msg)
	case focusGroups:
		m.updateGroups(msg)
	default:
		m.tree.Update(msg)
		m.layout()
	}
	return nil
}

func (m *Model) prependCall(call proxy.Call) {
	m.nextID++
	m.allCalls = append([]callItem{{call: call, id: m.nextID}}, m.allCalls...)
	if m.maxCalls > 0 && len(m.allCalls) > m.maxCalls {
		m.allCalls = m.allCalls[:m.maxCalls]
	}
}

// inputActive reports whether a text-entry mode is capturing keystrokes, so
// global shortcuts like "q" don't fire mid-typing.
func (m Model) inputActive() bool {
	return (m.focused == focusList && m.filtering) || (m.focused == focusDetail && m.tree.typing)
}

func (m *Model) advanceFocus(reverse bool) {
	m.filtering = false
	m.tree.clearFilter()
	order := []focusZone{focusList, focusDetail}
	if m.groupsVisible {
		order = []focusZone{focusList, focusGroups, focusDetail}
	}
	idx := 0
	for i, f := range order {
		if f == m.focused {
			idx = i
			break
		}
	}
	if reverse {
		idx = (idx - 1 + len(order)) % len(order)
	} else {
		idx = (idx + 1) % len(order)
	}
	m.focused = order[idx]
}

func (m *Model) clearHistory() {
	m.allCalls = nil
	m.filter = ""
	m.filtering = false
	m.groupMgr = NewGroupManager(nil)
	m.groupsVisible = false
	m.groupCursor = 0
	m.groupFilterID = ""
	m.focused = focusList
	m.tree.focused = false
	m.rebuildList()
	if m.OnClear != nil {
		m.OnClear()
	}
}

func (m *Model) syncTree() {
	sel, ok := m.list.SelectedItem().(callItem)
	if !ok {
		return
	}
	if sel.id == m.loadedID {
		return
	}
	m.loadedID = sel.id
	m.selected = sel.call
	m.hasSel = true
	m.tree.setCall(sel.call)
	m.layout()
}
