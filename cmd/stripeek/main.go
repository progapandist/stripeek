package main

import (
	"fmt"
	"net/http"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/progapandist/stripeek/proxy"
	"github.com/progapandist/stripeek/tui"
)

func main() {
	addr := "127.0.0.1:4111"
	if v := os.Getenv("STRIPEEK_ADDR"); v != "" {
		addr = v
	}

	calls := make(chan proxy.Call, 64)

	go func() {
		if err := http.ListenAndServe(addr, proxy.Handler(calls)); err != nil {
			fmt.Fprintf(os.Stderr, "proxy: %v\n", err)
			os.Exit(1)
		}
	}()

	m := tui.New()
	p := tea.NewProgram(m, tea.WithAltScreen())

	// Feed captured calls into the TUI as messages.
	go func() {
		for c := range calls {
			p.Send(tui.NewCallMsg(c))
		}
	}()

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "tui: %v\n", err)
		os.Exit(1)
	}
}
