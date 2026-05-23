package tui

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/progapandist/stripeek/proxy"
)

var (
	styleDim    = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	styleOK     = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	styleErr    = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	styleBorder = lipgloss.NewStyle().BorderStyle(lipgloss.NormalBorder()).BorderRight(true).BorderForeground(lipgloss.Color("238"))
)

// callItem wraps proxy.Call to satisfy the list.Item interface.
type callItem struct {
	call proxy.Call
}

func (c callItem) Title() string {
	status := styleOK.Render(fmt.Sprintf("%d", c.call.Status))
	if c.call.Status >= 400 {
		status = styleErr.Render(fmt.Sprintf("%d", c.call.Status))
	}
	return fmt.Sprintf("%s %s %s", c.call.Method, c.call.Path, status)
}

func (c callItem) Description() string {
	return styleDim.Render(fmt.Sprintf("%s  %dms", c.call.Time.Format("15:04:05"), c.call.Latency.Milliseconds()))
}

func (c callItem) FilterValue() string { return c.call.Path }

// NewCallMsg carries a newly captured call into the TUI.
type NewCallMsg proxy.Call

// Model is the root Bubble Tea model.
type Model struct {
	list    list.Model
	detail  viewport.Model
	width   int
	height  int
	focused string // "list" | "detail"
}

func New() Model {
	delegate := list.NewDefaultDelegate()
	l := list.New(nil, delegate, 0, 0)
	l.Title = "stripeek"
	l.SetShowStatusBar(true)
	l.SetFilteringEnabled(true)

	vp := viewport.New(0, 0)

	return Model{
		list:    l,
		detail:  vp,
		focused: "list",
	}
}

func (m Model) Init() tea.Cmd { return nil }

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		listW := msg.Width / 2
		detailW := msg.Width - listW - 1
		h := msg.Height - 2
		m.list.SetSize(listW, h)
		m.detail.Width = detailW
		m.detail.Height = h

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "tab":
			if m.focused == "list" {
				m.focused = "detail"
			} else {
				m.focused = "list"
			}
		}

	case NewCallMsg:
		m.list.InsertItem(0, callItem{call: proxy.Call(msg)})
		m.updateDetail()
	}

	var cmd tea.Cmd
	if m.focused == "list" {
		m.list, cmd = m.list.Update(msg)
		m.updateDetail()
	} else {
		m.detail, cmd = m.detail.Update(msg)
	}
	return m, cmd
}

func (m *Model) updateDetail() {
	sel, ok := m.list.SelectedItem().(callItem)
	if !ok {
		return
	}
	c := sel.call
	var sb strings.Builder
	fmt.Fprintf(&sb, "▶ %s %s  %d  %dms\n\n", c.Method, c.Path, c.Status, c.Latency.Milliseconds())
	fmt.Fprintf(&sb, "── Request Body ──\n%s\n\n", prettyJSON(c.ReqBody))
	fmt.Fprintf(&sb, "── Response Body ──\n%s\n", prettyJSON(c.RespBody))
	m.detail.SetContent(sb.String())
}

func (m Model) View() string {
	if m.width == 0 {
		return "loading…"
	}
	listW := m.width / 2
	left := styleBorder.Width(listW).Render(m.list.View())
	right := m.detail.View()
	return lipgloss.JoinHorizontal(lipgloss.Top, left, right)
}

func prettyJSON(b []byte) string {
	if len(b) == 0 {
		return styleDim.Render("(empty)")
	}
	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		// Stripe form-encoded bodies aren't JSON; return as-is.
		return string(b)
	}
	out, _ := json.MarshalIndent(v, "", "  ")
	return string(out)
}
