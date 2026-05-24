package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/progapandist/stripeek/history"
	"github.com/progapandist/stripeek/proxy"
	"github.com/progapandist/stripeek/tui"
)

// Set by goreleaser via -ldflags.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	for _, arg := range os.Args[1:] {
		if arg == "--version" || arg == "-version" {
			fmt.Printf("stripeek %s (commit %s, built %s)\n", version, commit, date)
			os.Exit(0)
		}
	}

	addr := "127.0.0.1:4242"
	if v := os.Getenv("STRIPEEK_ADDR"); v != "" {
		addr = v
	}
	historyLimit := envInt("STRIPEEK_HISTORY_LIMIT", tui.DefaultMaxCalls)
	historyPath := os.Getenv("STRIPEEK_HISTORY_PATH")
	if historyPath == "" {
		historyPath = history.DefaultPath()
	}
	store := history.New(historyPath, historyLimit)
	savedCalls, err := store.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "history load: %v\n", err)
	}

	calls := make(chan proxy.Call, 64)
	server := &http.Server{
		Addr:              addr,
		Handler:           proxy.Handler(calls),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       90 * time.Second,
	}

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Fprintf(os.Stderr, "proxy: %v\n", err)
			os.Exit(1)
		}
	}()

	groups := tui.NewGroupManager(savedCalls)
	m := tui.NewWithGroupManager(historyLimit, savedCalls, groups)
	m.OnClear = func() {
		if err := store.Clear(); err != nil {
			fmt.Fprintf(os.Stderr, "history clear: %v\n", err)
		}
	}
	p := tea.NewProgram(m, tea.WithAltScreen())

	// Feed captured calls into the TUI as messages.
	go func() {
		for c := range calls {
			if group := groups.Current(); group != nil && !c.Time.Before(group.StartedAt) {
				c.Group = group
			}
			if err := store.Append(c); err != nil {
				fmt.Fprintf(os.Stderr, "history append: %v\n", err)
			}
			p.Send(tui.NewCallMsg(c))
		}
	}()

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "tui: %v\n", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "proxy shutdown: %v\n", err)
	}
}

func envInt(name string, fallback int) int {
	v := os.Getenv(name)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s=%q is not an integer; using %d\n", name, v, fallback)
		return fallback
	}
	return n
}
