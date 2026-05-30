package tui

import (
	kb "github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

// Key bindings are defined once here and matched with kb.Matches throughout the
// update loop, so an alias like cmd+g for ctrl+g lives in exactly one place
// instead of being repeated as `msg.String() == "ctrl+g" || …` at every call
// site. Display/help text intentionally stays in shortcuts.go and helpBar, which
// describe key *groups* with their own wording per focus state.
//
// The bubbles/key package is imported as kb so its name doesn't collide with the
// test helper named key.
var (
	// Global — handled before any pane sees the key.
	keyForceQuit    = kb.NewBinding(kb.WithKeys("ctrl+c"))
	keyQuit         = kb.NewBinding(kb.WithKeys("q"))
	keyShortcuts    = kb.NewBinding(kb.WithKeys("?"))
	keyDismiss      = kb.NewBinding(kb.WithKeys("esc"))
	keyNextPane     = kb.NewBinding(kb.WithKeys("tab"))
	keyPrevPane     = kb.NewBinding(kb.WithKeys("shift+tab"))
	keyNewGroup     = kb.NewBinding(kb.WithKeys("ctrl+g", "cmd+g"))
	keyClearHistory = kb.NewBinding(kb.WithKeys("ctrl+x"))

	// Shared navigation, used by the call list and the group navigator.
	keyUp     = kb.NewBinding(kb.WithKeys("up", "k"))
	keyDown   = kb.NewBinding(kb.WithKeys("down", "j"))
	keyTop    = kb.NewBinding(kb.WithKeys("t", "home"))
	keyBottom = kb.NewBinding(kb.WithKeys("b", "end"))
	keyPageUp = kb.NewBinding(kb.WithKeys("pgup", "ctrl+b"))
	keyPageDn = kb.NewBinding(kb.WithKeys("pgdown", "ctrl+f"))
	keyHalfUp = kb.NewBinding(kb.WithKeys("ctrl+u"))
	keyHalfDn = kb.NewBinding(kb.WithKeys("ctrl+d"))

	// Pane-specific actions.
	keyInspect      = kb.NewBinding(kb.WithKeys("enter"))
	keyFilter       = kb.NewBinding(kb.WithKeys("/"))
	keyToggleGroups = kb.NewBinding(kb.WithKeys("g"))
)

// matches reports whether msg triggers binding b.
func matches(msg tea.KeyMsg, b kb.Binding) bool {
	return kb.Matches(msg, b)
}
