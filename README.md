# stripeek

Debugging a Stripe integration locally usually means guessing what the SDK actually sends, sprinkling log statements, or opening the Stripe Dashboard after the fact. stripeek shows you the full request/response pair as it happens — headers, body, status, and latency — without touching your application code. Stripe responses are deeply nested JSON, and stripeek lets you navigate and filter that structure interactively, which is far faster than squinting at raw logs.

stripeek runs as a reverse proxy between your application and `api.stripe.com`. Point your Stripe SDK at `http://localhost:4242` and every request/response pair appears in a browsable TUI — with full JSON inspection, request grouping, persistent history, and clickable Stripe Dashboard links.

Stripe integrations are two-way: your app calls the API, and Stripe calls *you* back with webhooks. stripeek captures **both sides in the same timeline** — it can also sit in front of your local webhook endpoint, so the events Stripe sends appear alongside the API calls that triggered them, and you can [relate a request to the webhooks it produced](#inspecting-webhooks) with a single keystroke.

> **For local development only.**  
> Redirecting your SDK's base URL to stripeek means your app routes all Stripe traffic through the proxy. If stripeek isn't running, every Stripe API call will fail. Never commit these changes or deploy them to staging or production — keep them in local dev overrides (environment-specific initializers, `.env.development`, or a dev-only boot file).

<img width="1320" height="820" alt="stripeek" src="https://github.com/user-attachments/assets/27e34b63-096d-40ac-a6d6-77fce6f5d96b" />

Outbound API calls (`▶`) and inbound webhooks (`◀`) share one timeline. The inspector shows the full request/response pair, with JSON navigation and filtering. The header toggle (`h`) lets you fold the raw HTTP headers in and out of the tree. Pressing `r` on any call or event focuses the related calls and webhooks — the API call that caused an event and every event it produced — so you can see the whole operation in one place, even if the events don't follow the request directly. `ctrl-r` does the same but dims the unrelated calls instead of hiding them, so you can keep the timeline context.


## Installation

```bash
go install github.com/progapandist/stripeek/cmd/stripeek@latest
```

Requires Go 1.24 or later. The binary lands in `$(go env GOPATH)/bin` — make sure that's on your `$PATH`. You can also download the binaries for different platforms from the release page. Easier distributions like a Homebrew formula will be added at the later stage when the feature set somewhat stabilizes.

> **Active development.**
> The tool is under the active development and new features/improvements land often on main before being included in a tagged release, prefer `go install github.com/progapandist/stripeek/cmd/stripeek@main` to install directly from the main branch.

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

## App setup example (Rails, but the similar approach will apply for other frameworks) 

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

## Inspecting webhooks

Stripe integrations don't end at the API call — Stripe calls your app back with webhook events. stripeek can capture those too, so the events Stripe delivers show up in the same TUI as the API requests that set them off.

Run stripeek with a webhook target pointing at the local server that processes your Stripe webhooks. You only give the server's base URL — there's no need to specify the exact webhook endpoint:

```bash
STRIPEEK_WEBHOOK_TARGET=http://localhost:4567 stripeek
```

Then point `stripe listen` at stripeek's webhook listener (port `4243` by default). The path you put here is the path stripeek forwards to, so with `/webhooks` your local server receives events at `http://localhost:4567/webhooks`:

```bash
stripe listen --forward-to localhost:4243/webhooks
```

Now every incoming webhook appears in stripeek's TUI in real time, marked with a `◀` glyph and the event name. The body and `Stripe-Signature` header are forwarded untouched, so your app's signature verification still passes.

### Relating requests and webhooks

Select any request or event and press `r` to group the related requests and webhooks together — the API call that caused an event and every event it produced (`ctrl+r` dims the rest instead of hiding it). The relation is inferred from a combination of the Stripe `request_id` and object IDs (subscription, customer, invoice, …), so even webhooks that don't follow a request directly — for example the events from a subscription change when a test clock advances — are correctly attributed to the operation that set them in motion.

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
| `r` / `ctrl+r` | Focus related request + webhooks / dim the rest (`esc` exits) |
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

Groups allow you to visually cluster the requests which can be useful when you want to quickly grock all the traffic related to testing a certain feature (e.g., all the requests you make to Stripe when your app's admin interface loads the subscription view). To group the upcoming requests automatically hit `ctrl+g` and the new group will be created and assigned a random name based on color of the markers that will visually separate the requests in the TUI.

<img width="1496" height="1075" alt="image" src="https://github.com/user-attachments/assets/643a84e4-ff69-4706-959b-35b5e313feae" />


| Key | Action |
|---|---|
| `g` | Open / close groups panel |
| `ctrl+g` | Start a new group |
| `enter` | Show calls for selected group |
| `esc` | Show all requests |

The inspector header shows a `TEST` / `LIVE` badge for each call, inferred from the API key prefix (the key itself is never stored).

Stripe object IDs in the inspector are rendered as clickable hyperlinks to the Stripe Dashboard (requires a terminal with OSC 8 support — iTerm2, WezTerm, Kitty). Links follow the call's mode, so a live request opens the live dashboard instead of the test one.

## Configuration

| Variable | Default | Description |
|---|---|---|
| `STRIPEEK_ADDR` | `127.0.0.1:4242` | Address the outbound Stripe API proxy listens on |
| `STRIPEEK_WEBHOOK_TARGET` | _(unset)_ | Local app URL to forward inbound Stripe webhooks to. Setting it enables the webhook listener; leave unset to disable webhook capture |
| `STRIPEEK_WEBHOOK_ADDR` | `127.0.0.1:4243` | Address the inbound webhook listener listens on (point `stripe listen --forward-to` here) |
| `STRIPEEK_HISTORY_LIMIT` | `1000` | Maximum number of calls kept in memory and on disk |
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
