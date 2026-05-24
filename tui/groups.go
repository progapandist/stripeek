package tui

import (
	"fmt"
	"sync"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/progapandist/stripeek/proxy"
)

type groupColor struct {
	name  string
	light string
	dark  string
}

var groupPalette = []groupColor{
	{name: "Teal", light: "#0f766e", dark: "#5eead4"},
	{name: "Amber", light: "#b45309", dark: "#fbbf24"},
	{name: "Rose", light: "#be123c", dark: "#fb7185"},
	{name: "Blue", light: "#2563eb", dark: "#93c5fd"},
	{name: "Emerald", light: "#047857", dark: "#86efac"},
	{name: "Fuchsia", light: "#c026d3", dark: "#f0abfc"},
	{name: "Cyan", light: "#0891b2", dark: "#67e8f9"},
	{name: "Lime", light: "#4d7c0f", dark: "#bef264"},
	{name: "Pink", light: "#db2777", dark: "#f9a8d4"},
	{name: "Indigo", light: "#4f46e5", dark: "#a5b4fc"},
	{name: "Orange", light: "#ea580c", dark: "#fdba74"},
	{name: "Sky", light: "#0284c7", dark: "#7dd3fc"},
	{name: "Green", light: "#15803d", dark: "#4ade80"},
	{name: "Red", light: "#dc2626", dark: "#f87171"},
	{name: "Violet", light: "#7c3aed", dark: "#c4b5fd"},
	{name: "Yellow", light: "#a16207", dark: "#fde047"},
	{name: "Slate", light: "#475569", dark: "#cbd5e1"},
	{name: "Stone", light: "#57534e", dark: "#d6d3d1"},
}

// GroupManager stores the active request group shared by the TUI and capture
// pipeline. It is safe for the Bubble Tea update loop and proxy goroutine to
// access concurrently.
type GroupManager struct {
	mu     sync.RWMutex
	active *proxy.Group
	next   int
}

func NewGroupManager(saved []proxy.Call) *GroupManager {
	seen := map[string]bool{}
	for _, c := range saved {
		if c.Group != nil && c.Group.ID != "" {
			seen[c.Group.ID] = true
		}
	}
	return &GroupManager{next: len(seen)}
}

func (g *GroupManager) Current() *proxy.Group {
	if g == nil {
		return nil
	}
	g.mu.RLock()
	defer g.mu.RUnlock()
	return cloneGroup(g.active)
}

func (g *GroupManager) Start() proxy.Group {
	if g == nil {
		return proxy.Group{}
	}
	g.mu.Lock()
	defer g.mu.Unlock()

	idx := g.next
	color := groupPalette[idx%len(groupPalette)]
	cycle := idx / len(groupPalette)
	name := "Group " + color.name
	if cycle > 0 {
		name = fmt.Sprintf("%s %d", name, cycle+1)
	}
	now := time.Now()
	group := &proxy.Group{
		ID:        fmt.Sprintf("group-%d-%d", now.UnixNano(), idx+1),
		Name:      name,
		Color:     color.name,
		LightHex:  color.light,
		DarkHex:   color.dark,
		StartedAt: now,
	}
	g.active = group
	g.next++
	return *cloneGroup(group)
}

func cloneGroup(g *proxy.Group) *proxy.Group {
	if g == nil {
		return nil
	}
	cp := *g
	return &cp
}

func groupStyle(g *proxy.Group) lipgloss.Style {
	if g == nil {
		return styleDim
	}
	return lipgloss.NewStyle().Foreground(adaptive(g.LightHex, g.DarkHex))
}
