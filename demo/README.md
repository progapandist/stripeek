# Demo

The README animation is recorded with [VHS](https://github.com/charmbracelet/vhs)
from a real captured Stripe run, so it needs no live Stripe key, traffic, or network.

## Files

- **`stripeek.tape`** — the VHS script (the recipe). Committed.
- **`demo-history.json`** — a captured run loaded via `STRIPEEK_HISTORY_PATH`,
  so the TUI opens pre-populated. Secrets are already redacted in stripeek's
  stored history (no API keys); the data is synthetic test-mode traffic with two
  manual groups pre-assigned. Committed.
- **`stripeek.gif`** — the rendered output. **Not committed** (it's hosted as a
  GitHub attachment to keep the repo lean and avoid binary churn). Git-ignored.

## Regenerate

```bash
brew install vhs gifsicle        # vhs pulls in ttyd + ffmpeg
go build -o stripeek ./cmd/stripeek
vhs demo/stripeek.tape           # writes demo/stripeek.gif
gifsicle -O3 --lossy=60 demo/stripeek.gif -o demo/stripeek.gif
```

The tape copies `demo-history.json` to a temp file before launching, so recording
never mutates the committed fixture.

## Publish

Drag `demo/stripeek.gif` into a GitHub comment, issue, or release-edit box to mint
an attachment URL, then paste it over `DEMO_GIF_URL` in the top-level `README.md`.
