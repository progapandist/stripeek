# stripeek demo — regenerate with:  vhs demo/stripeek.tape
#
# Drives the TUI over a real captured Stripe run (demo/demo-history.json) loaded
# from disk, so the demo needs no live Stripe traffic, key, or network. The
# fixture is copied to a temp file first so recording never mutates the
# committed one. Build the binary before recording:  go build -o stripeek ./cmd/stripeek

Output demo/stripeek.gif

Set Shell "bash"
Set FontSize 14
Set Width 1320
Set Height 820
Set Padding 16
Set TypingSpeed 80ms

# --- hidden setup: load the captured run, no live traffic needed ---
Hide
Type "cp demo/demo-history.json /tmp/stripeek-demo.json"
Enter
Type "STRIPEEK_HISTORY_PATH=/tmp/stripeek-demo.json STRIPEEK_ADDR=127.0.0.1:4252 ./stripeek"
Enter
Sleep 2.5s
Show

# Captured run opens pre-populated: outbound ▶ calls and inbound ◀ webhooks,
# colored by manual group, newest first.
Sleep 2s

# Walk the list — the inspector previews each request/response as you move.
Down@450ms 4
Sleep 1.2s
Up@450ms 2
Sleep 1s

# Inspect a call: focus the JSON tree.
Enter
Sleep 1.5s

# Navigate and expand the deeply nested response.
Down@300ms 5
Sleep 800ms
Right
Sleep 1.2s
# Collapse everything, then expand it all back.
Type "-"
Sleep 1.2s
Type "+"
Sleep 1.5s

# Toggle raw HTTP headers in and out of the tree.
Type "h"
Sleep 2s
Type "h"
Sleep 1s
Escape
Sleep 800ms

# Filter the list by path / event name — narrow to invoice activity.
Type "/"
Type "invoice"
Sleep 2s
Escape
Sleep 800ms

# Webhook relations: land on an event, then focus its whole operation.
Type "/"
Type "subscription.created"
Sleep 1.5s
Escape
Sleep 600ms
# r — show only the API call that caused this event and every event it produced.
Type "r"
Sleep 3s
Escape
Sleep 800ms
# ctrl+r — same operation, but dim the rest instead of hiding it (timeline kept).
Ctrl+R
Sleep 3s
Escape
Sleep 800ms

# Manual groups: open the navigator and filter by a named group.
Type "g"
Sleep 1s
Tab
Sleep 600ms
Down
Sleep 2s
Down
Sleep 2s
Escape
Sleep 800ms
Type "g"
Sleep 800ms

# Full keymap.
Type "?"
Sleep 3s
Escape
Sleep 1.2s

# Quit off-camera so the recording ends on the TUI, not the shell teardown
# (exiting the alt-screen would otherwise reveal the hidden setup commands).
Hide
Type "q"
Sleep 500ms
