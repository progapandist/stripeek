package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"sync/atomic"
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
	fs := flag.NewFlagSet("stripeek", flag.ExitOnError)
	showVersion := fs.Bool("version", false, "print version and exit")
	addrFlag := fs.String("addr", envOr("STRIPEEK_ADDR", "127.0.0.1:4242"),
		"address for the outbound Stripe API proxy")
	webhookTarget := fs.String("webhook-target", os.Getenv("STRIPEEK_WEBHOOK_TARGET"),
		"forward inbound Stripe CLI webhooks to this local app URL (enables the webhook listener)")
	webhookAddr := fs.String("webhook-addr", envOr("STRIPEEK_WEBHOOK_ADDR", "127.0.0.1:4243"),
		"address for the inbound webhook listener")
	// Ignore parse errors beyond flag's own ExitOnError handling; unknown args
	// shouldn't crash a debugging tool.
	_ = fs.Parse(os.Args[1:])

	if *showVersion {
		fmt.Printf("stripeek %s (commit %s, built %s)\n", version, commit, date)
		os.Exit(0)
	}

	addr := *addrFlag
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
	var dropped atomic.Int64
	server := &http.Server{
		Addr:              addr,
		Handler:           proxy.Handler(calls, proxy.WithDropCounter(&dropped)),
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

	// Optional second listener: forwards inbound Stripe CLI webhooks to a local
	// app, tagging each capture as a webhook. Only started when a target is set.
	var webhookServer *http.Server
	if *webhookTarget != "" {
		webhookServer = &http.Server{
			Addr: *webhookAddr,
			Handler: proxy.Handler(calls,
				proxy.WithTarget(*webhookTarget),
				proxy.WithWebhook(),
				proxy.WithDropCounter(&dropped)),
			ReadHeaderTimeout: 5 * time.Second,
			ReadTimeout:       30 * time.Second,
			WriteTimeout:      60 * time.Second,
			IdleTimeout:       90 * time.Second,
		}
		go func() {
			if err := webhookServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				fmt.Fprintf(os.Stderr, "webhook proxy: %v\n", err)
			}
		}()
	}

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
		var lastDropped int64
		for c := range calls {
			if group := groups.Current(); group != nil && !c.Time.Before(group.StartedAt) {
				c.Group = group
			}
			if err := store.Append(c); err != nil {
				fmt.Fprintf(os.Stderr, "history append: %v\n", err)
			}
			p.Send(tui.NewCallMsg(c))
			// Surface any captures dropped while the consumer was behind.
			if d := dropped.Load(); d != lastDropped {
				lastDropped = d
				p.Send(tui.DroppedMsg(d))
			}
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
	if webhookServer != nil {
		if err := webhookServer.Shutdown(ctx); err != nil {
			fmt.Fprintf(os.Stderr, "webhook proxy shutdown: %v\n", err)
		}
	}
}

func envOr(name, fallback string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return fallback
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
