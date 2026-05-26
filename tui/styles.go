package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// This file is the single source of truth for the TUI's look: the color
// palette and every lipgloss style derived from it. Keeping it isolated means
// re-theming the app is a matter of editing one file, and the rest of the
// package refers to styles by intent (styleHelpKey) rather than raw colors.
//
// Colors are lipgloss.AdaptiveColor so they keep good contrast on both light
// and dark terminals (lipgloss picks Light/Dark by detecting the background).
// Hex values are used instead of 256-color indices because indices render
// differently across themes — #af87ff on a muted dark background can wash out,
// whereas a fixed hex stays put.

// adaptive is a small constructor to keep the palette block readable.
func adaptive(light, dark string) lipgloss.AdaptiveColor {
	return lipgloss.AdaptiveColor{Light: light, Dark: dark}
}

// Palette — one violet accent (focus, keys, selection) over neutral grays,
// with green/red reserved for HTTP status. JSON scalars get their own hues.
var (
	colAccent     = adaptive("#6d28d9", "#b794f6") // violet: focus, keys, selection
	colAccentSoft = adaptive("#7c5cc4", "#9f8fd8") // muted violet: secondary text
	colBorder     = adaptive("#c4c4cc", "#52525b") // inactive pane border / rules
	colTitleDim   = adaptive("#6b6b76", "#a1a1aa") // inactive pane title
	colDim        = adaptive("#6b6b76", "#9a9aa4") // metadata / secondary text
	colFaint      = adaptive("#a8a8b0", "#5c5c66") // placeholders / null / very faint
	colOK         = adaptive("#15803d", "#4ade80") // 2xx status
	colErr        = adaptive("#dc2626", "#f87171") // >=400 status

	// Selection highlight: a saturated fill with near-white text, legible on
	// any background (the tree cursor and any future selection bar use it).
	colSelBg = adaptive("#7c3aed", "#4338ca")
	colSelFg = adaptive("#ffffff", "#f5f3ff")

	colString = adaptive("#15803d", "#87d787") // JSON string (green)
	colNumber = adaptive("#b45309", "#f0a868") // JSON number (orange)
	colBool   = adaptive("#4f46e5", "#a5b4fc") // JSON bool (periwinkle)
	colKey    = adaptive("#0369a1", "#7dd3fc") // JSON object key (blue)

	// HTTP headers get their own teal family so they read as metadata, clearly
	// set apart from JSON payload fields when shown in the same tree.
	colHeaderKey = adaptive("#0e7490", "#5eead4") // header name / section (teal)
	colHeaderVal = adaptive("#0f766e", "#67c9be") // header value (muted teal)

	// HTTP method classes: safe/read methods stay cool, mutating ones run warm
	// so a glance down the Calls list separates reads from writes.
	colMethodSafe  = adaptive("#0e7490", "#67e8f9") // GET/HEAD/OPTIONS/TRACE (cyan)
	colMethodWrite = adaptive("#c2410c", "#fb923c") // POST/PUT/PATCH/DELETE/… (amber)
)

// Shared text styles.
var (
	styleDim   = lipgloss.NewStyle().Foreground(colDim)
	styleFaint = lipgloss.NewStyle().Foreground(colFaint)
	styleOK    = lipgloss.NewStyle().Foreground(colOK)
	styleErr   = lipgloss.NewStyle().Foreground(colErr)
)

// Chrome: pane titles, footer help, filter input.
var (
	styleTitleActive   = lipgloss.NewStyle().Bold(true).Foreground(colAccent)
	styleTitleInactive = lipgloss.NewStyle().Bold(true).Foreground(colTitleDim)

	styleHelp        = lipgloss.NewStyle().Foreground(colDim)
	styleHelpKey     = lipgloss.NewStyle().Foreground(colAccent)
	styleFilterText  = lipgloss.NewStyle().Foreground(colAccent)
	stylePath        = lipgloss.NewStyle().Foreground(colAccentSoft)
	styleAccentBlock = lipgloss.NewStyle().Background(colAccent)
	styleScrollTrack = lipgloss.NewStyle().Foreground(colFaint)
	styleScrollThumb = lipgloss.NewStyle().Foreground(colAccent)

	styleMethodSafe  = lipgloss.NewStyle().Bold(true).Foreground(colMethodSafe)
	styleMethodWrite = lipgloss.NewStyle().Bold(true).Foreground(colMethodWrite)
)

// safeMethods are the HTTP methods with no side effects (RFC 9110 §9.2.1).
var safeMethods = map[string]struct{}{
	"GET": {}, "HEAD": {}, "OPTIONS": {}, "TRACE": {},
}

// methodStyle colors an HTTP method by whether it mutates server state.
func methodStyle(method string) lipgloss.Style {
	if _, ok := safeMethods[strings.ToUpper(method)]; ok {
		return styleMethodSafe
	}
	return styleMethodWrite
}

// Inspector JSON tree.
var (
	styleKey           = lipgloss.NewStyle().Foreground(colKey)
	styleHeaderKey     = lipgloss.NewStyle().Foreground(colHeaderKey)
	styleMarker        = lipgloss.NewStyle().Foreground(colDim)
	styleCursor        = lipgloss.NewStyle().Background(colSelBg).Foreground(colSelFg)
	styleMatch         = lipgloss.NewStyle().Foreground(colAccent).Bold(true)
	styleSectionHeader = lipgloss.NewStyle().Bold(true).Foreground(colAccent)
	styleHeaderSection = lipgloss.NewStyle().Bold(true).Foreground(colHeaderKey)

	colorString      lipgloss.TerminalColor = colString
	colorNumber      lipgloss.TerminalColor = colNumber
	colorBool        lipgloss.TerminalColor = colBool
	colorNull        lipgloss.TerminalColor = colFaint
	colorHeaderValue lipgloss.TerminalColor = colHeaderVal
)
