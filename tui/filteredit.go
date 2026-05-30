package tui

import tea "github.com/charmbracelet/bubbletea"

// filterEdit classifies what a keystroke means while editing a filter query —
// the path filter in the call list or the key filter in the inspector. Both
// filters are edited the same way (append runes, backspace, enter to apply, esc
// to abandon); only the side effects of each action differ, so this stays a pure
// classifier and each caller applies its own refresh.
type filterEdit int

const (
	editNone   filterEdit = iota // keystroke isn't part of filter editing
	editChange                   // query changed (rune appended or rune deleted)
	editCommit                   // finish editing, keep the query (enter)
	editCancel                   // abandon editing (esc)
)

// editFilterQuery interprets msg against the current query and returns the
// updated query plus what the keystroke meant. It only rewrites the query on
// rune input and backspace; commit and cancel leave it untouched so each caller
// decides whether to keep or clear it. A backspace on an empty query is a no-op
// (editNone) so callers don't re-run their (potentially costly) refresh.
func editFilterQuery(query string, msg tea.KeyMsg) (string, filterEdit) {
	switch msg.String() {
	case "enter":
		return query, editCommit
	case "esc":
		return query, editCancel
	case "backspace", "ctrl+h":
		r := []rune(query)
		if len(r) == 0 {
			return query, editNone
		}
		return string(r[:len(r)-1]), editChange
	default:
		if msg.Type == tea.KeyRunes {
			return query + string(msg.Runes), editChange
		}
		return query, editNone
	}
}
