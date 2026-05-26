# stripeek

Debugging a Stripe integration locally usually means guessing what the SDK actually sends, sprinkling log statements, or opening the Stripe Dashboard after the fact. stripeek shows you the full request/response pair as it happens — headers, body, status, and latency — without touching your application code. Stripe responses are deeply nested JSON, and stripeek lets you navigate and filter that structure interactively, which is far faster than squinting at raw logs.

stripeek runs as a reverse proxy between your application and `api.stripe.com`. Point your Stripe SDK at `http://localhost:4242` and every request/response pair appears in a browsable TUI — with full JSON inspection, request grouping, persistent history, and clickable Stripe Dashboard links.

> **For local development only.**  
> Redirecting your SDK's base URL to stripeek means your app routes all Stripe traffic through the proxy. If stripeek isn't running, every Stripe API call will fail. Never commit these changes or deploy them to staging or production — keep them in local dev overrides (environment-specific initializers, `.env.development`, or a dev-only boot file).

<img width="1679" height="1014" alt="image" src="https://github.com/user-attachments/assets/b8071094-0983-43e0-81de-10a37a12196a" />

With request groups and filtering: 
<img width="1589" height="874" alt="image" src="https://github.com/user-attachments/assets/2d989c3d-d952-4cc3-8c51-3e5ebc38e4be" />


## Installation

```bash
go install github.com/progapandist/stripeek/cmd/stripeek@latest
```

Requires Go 1.24 or later. The binary lands in `$(go env GOPATH)/bin` — make sure that's on your `$PATH`.

## Quick start

```bash
stripeek          # listens on http://localhost:4242
```

Then redirect your Stripe SDK to the proxy — **in your local dev environment only**. Stripe calls will fail if stripeek isn't running after this change, so guard it behind an environment check:

**Ruby**
```ruby
# config/initializers/stripe.rb (or equivalent dev-only file)
if Rails.env.development?
  Stripe.api_base = "http://localhost:4242"
end
```

**Python**
```python
import os
if os.getenv("APP_ENV") == "development":
    stripe.api_base = "http://localhost:4242"
```

**Node.js**
```js
const stripe = new Stripe(process.env.STRIPE_SECRET_KEY, {
  ...(process.env.NODE_ENV === "development" && {
    host: "localhost",
    port: 4242,
    protocol: "http",
  }),
});
```

**Go**
```go
if os.Getenv("APP_ENV") == "development" {
    stripe.SetAPIBase("http://localhost:4242")
}
```

stripeek proxies every request to the real Stripe API and captures the full request/response pair, including headers, body, status code, latency, and Stripe request ID. Your keys are redacted from captured headers automatically.

## Rails example

`config/initializers/stripe.rb`:
```ruby
Stripe.api_key = ENV.fetch("STRIPE_SECRET_KEY")

# Route traffic through stripeek when developing locally.
# Requires `stripeek` to be running — Stripe calls will fail without it.
if Rails.env.development? && ENV.key?("STRIPE_PROXY_URL")
  Stripe.api_base = ENV.fetch("STRIPE_PROXY_URL")
end
```

Start your server with the variable set to enable proxying:
```bash
STRIPE_PROXY_URL=http://localhost:4242 bin/rails server
```

Omit the variable (or start the server normally) to talk to Stripe directly. Production and staging are never affected — the block only runs in the development environment.

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
| `←→` | Fold / expand node |
| `space` / `enter` | Toggle container |
| `+` / `-` | Expand / collapse all |
| `h` | Toggle request / response headers |
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
| `STRIPEEK_HISTORY_PATH` | `os.TempDir()/stripeek-calls.json` | Where call history is persisted between sessions (`$TMPDIR` on macOS, `/tmp` on Linux) |

## Contributing

```bash
git clone https://github.com/progapandist/stripeek
cd stripeek
make build     # compile
make check     # fmt + vet + lint + build
go test ./...  # run tests
```

### Release process

Releases are automated via [goreleaser](https://goreleaser.com) and GitHub Actions. Pushing a semver tag to `main` triggers a build for all platforms and a GitHub release with attached archives and checksums.

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
