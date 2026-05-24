package tui

import (
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/progapandist/stripeek/proxy"
)

const DefaultMaxCalls = 100

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
		m.nextID++
		m.allCalls = append([]callItem{{call: c, id: m.nextID}}, m.allCalls...)
		if m.maxCalls > 0 && len(m.allCalls) > m.maxCalls {
			m.allCalls = m.allCalls[:m.maxCalls]
		}
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
		// ctrl+c always quits; q quits unless the user is typing (list filter
		// or inspector key search).
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "q":
			if !m.inputActive() {
				return m, tea.Quit
			}
		}
		if (msg.String() == "ctrl+g" || msg.String() == "cmd+g") && !m.inputActive() {
			m.startGroup()
			m.tree.focused = m.focused == focusDetail
			return m, nil
		}

		// Tab always switches panes/sections, cancelling any in-progress text input.
		if msg.String() == "tab" || msg.String() == "shift+tab" {
			m.advanceFocus(msg.String() == "shift+tab")
			m.tree.focused = m.focused == focusDetail
			return m, nil
		}
		// esc in the inspector returns to the list, unless a key filter is
		// active (then esc clears the filter — handled by the tree).
		if m.focused == focusDetail && msg.String() == "esc" && !m.tree.typing && !m.tree.filterOn {
			m.focused = focusList
			m.tree.focused = false
			return m, nil
		}

		switch m.focused {
		case focusList:
			cmd = m.updateList(msg)
		case focusGroups:
			m.updateGroups(msg)
		default:
			m.tree.Update(msg)
		}
		return m, cmd

	case NewCallMsg:
		followTop := len(m.list.Items()) == 0 || m.list.Index() == 0
		m.nextID++
		call := proxy.Call(msg)
		if call.Group == nil {
			if group := m.groupMgr.Current(); group != nil && !call.Time.Before(group.StartedAt) {
				call.Group = group
			}
		}
		item := callItem{call: call, id: m.nextID}
		m.allCalls = append([]callItem{item}, m.allCalls...)
		if m.maxCalls > 0 && len(m.allCalls) > m.maxCalls {
			m.allCalls = m.allCalls[:m.maxCalls]
		}
		m.rebuildList()
		if followTop && len(m.list.Items()) > 0 {
			m.list.Select(0)
			m.syncTree()
		}
		return m, nil
	}

	return m, cmd
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
}
