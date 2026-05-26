package tui

import "strings"

// linkContext carries the mode of the call whose payload is being rendered, so
// Dashboard links land in the matching live/test dashboard instead of always
// pointing at the test one.
type linkContext struct {
	mode string // "test", "live", or "" (treated as test)
}

// dashboardBase builds the Dashboard URL prefix for ctx: a "test/" segment
// unless the key is known to be live (unknown defaults to test, the common
// debugging case).
func (ctx linkContext) dashboardBase() string {
	base := "https://dashboard.stripe.com/"
	if ctx.mode != "live" {
		base += "test/"
	}
	return base
}

// stripeIDURL maps well-known Stripe ID prefixes to their Dashboard URLs.
// Longer prefixes must come first (sub_sched_ before sub_).
func stripeIDURL(id string, ctx linkContext) string {
	for _, e := range []struct{ prefix, path string }{
		{"sub_sched_", "subscription_schedules/"},
		{"cus_", "customers/"},
		{"pm_", "payment_methods/"},
		{"pi_", "payment_intents/"},
		{"price_", "prices/"},
		{"prod_", "products/"},
		{"sub_", "subscriptions/"},
		{"in_", "invoices/"},
		{"ch_", "charges/"},
		{"re_", "refunds/"},
		{"acct_", "connect/accounts/"},
		{"txn_", "balance/history/"},
	} {
		if strings.HasPrefix(id, e.prefix) {
			return ctx.dashboardBase() + e.path + id
		}
	}
	return ""
}

// scalarURL returns a clickable target for a scalar string: the value itself
// when it is already a URL, otherwise a Stripe Dashboard link for a known ID.
func scalarURL(s string, ctx linkContext) string {
	if strings.HasPrefix(s, "https://") || strings.HasPrefix(s, "http://") {
		return s
	}
	return stripeIDURL(s, ctx)
}

// hyperlink wraps text in an OSC 8 terminal hyperlink (iTerm2 / WezTerm / Kitty).
func hyperlink(u, text string) string {
	return "\x1b]8;;" + u + "\x07" + text + "\x1b]8;;\x07"
}
