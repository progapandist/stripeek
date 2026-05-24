# stripeek

A terminal UI for inspecting local Stripe API traffic in real time.

stripeek runs as a reverse proxy between your application and `api.stripe.com`. Point your Stripe SDK at `http://localhost:4242` and every request/response pair appears in a browsable TUI — with full JSON inspection, request grouping, persistent history, and clickable Stripe Dashboard links.

## Installation

### If you have Go installed

```bash
go install github.com/progapandist/stripeek/cmd/stripeek@latest
```

Requires Go 1.24 or later. The binary lands in `$(go env GOPATH)/bin` — make sure that's on your `$PATH`.

### Pre-built binaries (no Go required)

Download the archive for your platform from the [releases page](https://github.com/progapandist/stripeek/releases/latest).

**macOS (Apple Silicon)**
```bash
curl -L https://github.com/progapandist/stripeek/releases/latest/download/stripeek_darwin_arm64.tar.gz | tar xz
mv stripeek /usr/local/bin/
```

**macOS (Intel)**
```bash
curl -L https://github.com/progapandist/stripeek/releases/latest/download/stripeek_darwin_amd64.tar.gz | tar xz
mv stripeek /usr/local/bin/
```

`/usr/local/bin` is writable without `sudo` on most macOS setups (Homebrew ensures this). If you get a permissions error, use `~/bin` or any other directory on your `$PATH` instead.

**Linux (amd64)**
```bash
curl -L https://github.com/progapandist/stripeek/releases/latest/download/stripeek_linux_amd64.tar.gz | tar xz
mkdir -p ~/.local/bin && mv stripeek ~/.local/bin/
```

**Linux (arm64)**
```bash
curl -L https://github.com/progapandist/stripeek/releases/latest/download/stripeek_linux_arm64.tar.gz | tar xz
mkdir -p ~/.local/bin && mv stripeek ~/.local/bin/
```

Make sure `~/.local/bin` is on your `$PATH` (add `export PATH="$HOME/.local/bin:$PATH"` to your shell profile if needed).

**Windows**

Download `stripeek_windows_amd64.zip` from the [releases page](https://github.com/progapandist/stripeek/releases/latest), extract it, and place `stripeek.exe` somewhere on your `%PATH%`.

## Quick start

```bash
stripeek          # listens on http://localhost:4242
```

Then redirect your Stripe SDK to the proxy for the duration of your session:

**Ruby**
```ruby
Stripe.api_base = "http://localhost:4242"
```

**Python**
```python
stripe.api_base = "http://localhost:4242"
```

**Node.js**
```js
const stripe = new Stripe(process.env.STRIPE_SECRET_KEY, {
  host: "localhost",
  port: 4242,
  protocol: "http",
});
```

**Go**
```go
stripe.SetAPIBase("http://localhost:4242")
```

stripeek proxies every request to the real Stripe API and captures the full request/response pair, including headers, body, status code, latency, and Stripe request ID. Your keys are redacted from captured headers automatically.

## Keyboard shortcuts

| Key | Action |
|---|---|
| `tab` / `shift+tab` | Switch panes |
| `?` | Open / close shortcuts overlay |
| `ctrl+x` | Clear all history (memory + disk) |
| `q` / `ctrl+c` | Quit |

**Calls pane**

| Key | Action |
|---|---|
| `↑↓` / `j k` | Move one request |
| `pgup` / `pgdn`, `ctrl+b/f` | Move one page |
| `ctrl+u/d` | Move half page |
| `home` / `end`, `t` / `b` | Jump to top / bottom |
| `enter` | Inspect selected request |
| `/` / `esc` | Filter / clear filter |
| `g` / `ctrl+g` | Open groups / start new group |

**Inspector pane**

| Key | Action |
|---|---|
| `↑↓` / `j k` | Move one row |
| `←→` / `h l` | Fold / expand node |
| `space` / `enter` | Toggle container |
| `+` / `-` | Expand / collapse all |
| `/` / `esc` | Filter keys / back to list |
| `t` / `b` | Jump to top / bottom |

**Groups**

| Key | Action |
|---|---|
| `g` | Open / close groups panel |
| `ctrl+g` | Start a new group |
| `enter` | Show calls for selected group |
| `esc` | Show all requests |

Stripe object IDs in the inspector are rendered as clickable hyperlinks to the Stripe Dashboard (requires a terminal with OSC 8 support — iTerm2, WezTerm, Kitty).

## Configuration

| Variable | Default | Description |
|---|---|---|
| `STRIPEEK_ADDR` | `127.0.0.1:4242` | Address the proxy listens on |
| `STRIPEEK_HISTORY_LIMIT` | `100` | Maximum number of calls kept in memory and on disk |
| `STRIPEEK_HISTORY_PATH` | `$TMPDIR/stripeek-calls.json` | Where call history is persisted between sessions |

## Contributing

```bash
git clone https://github.com/progapandist/stripeek
cd stripeek
make build     # compile
make check     # fmt + vet + lint + build
go test ./...  # run tests
```

### Release process

Releases are fully automated via [goreleaser](https://goreleaser.com) and GitHub Actions. Pushing a semver tag to `main` triggers a build for all platforms, a GitHub release with attached archives and checksums, and an update to the Homebrew formula in `progapandist/homebrew-tap`.

**Versioning:** this project uses [Semantic Versioning](https://semver.org). While the major version is `0`, minor bumps (`v0.2.0`, `v0.3.0`) signal new features and patch bumps (`v0.1.1`) signal bug fixes. There are no compatibility guarantees before `v1.0.0`.

**To cut a release:**

```bash
# make sure you're on main and the working tree is clean
git checkout main
git pull

# create and push the tag — this is the only step required
git tag v0.2.0
git push origin v0.2.0
```

GitHub Actions runs goreleaser, which:
1. Compiles binaries for Linux (amd64, arm64), macOS (amd64, arm64), and Windows (amd64)
2. Creates a GitHub release with `tar.gz`/`zip` archives and `checksums.txt`

You can validate the goreleaser config locally without building:
```bash
make release-check
```

You can do a full local snapshot build (all platforms, no publishing) with:
```bash
make snapshot   # outputs to ./dist/
```
