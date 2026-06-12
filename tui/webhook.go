package tui

import (
	"encoding/json"
	"strings"
)

// webhookInfo is presentation metadata derived once from a forwarded Stripe
// event body, so rows and the inspector header never re-parse JSON per frame.
type webhookInfo struct {
	eventType string   // body `type`, e.g. "customer.subscription.created"
	eventID   string   // body `id`, evt_…
	livemode  string   // "live" or "test" (modeBadge vocabulary); "" when unknown
	requestID string   // req_… of the API call that caused the event; "" for async events
	refs      []string // object ids referenced by data.object, for operation adoption
}

// opRefFields are the object-reference keys harvested from a webhook's
// data.object and from an outbound call's top-level response body. They are the
// common denominators that chain async cascade events (request.id null) back to
// the mutation that started them. Deliberately shallow: deep body walks would
// pull unrelated ids out of list responses.
var opRefFields = []string{
	"id", "customer", "subscription", "test_clock", "invoice",
	"payment_intent", "latest_invoice",
}

// opRefPrefixes are the Stripe ID shapes recognised when seeding an operation
// from an outbound request URI (path segments and query values). Longer
// prefixes first so sub_sched_ doesn't read as sub_.
var opRefPrefixes = []string{
	"sub_sched_", "cus_", "sub_", "in_", "pi_", "price_", "prod_", "pm_",
	"ch_", "re_", "seti_", "clock_", "acct_", "txn_",
}

// webhookMeta pulls event metadata from a forwarded Stripe event body. Returns
// the zero value when the body isn't a recognisable event (including when
// truncation broke the JSON, which is fine — callers fall back to the request
// path).
func webhookMeta(body []byte) webhookInfo {
	var e struct {
		ID       string          `json:"id"`
		Type     string          `json:"type"`
		Livemode *bool           `json:"livemode"`
		Request  json.RawMessage `json:"request"`
		Data     struct {
			Object map[string]json.RawMessage `json:"object"`
		} `json:"data"`
	}
	if json.Unmarshal(body, &e) != nil {
		return webhookInfo{}
	}
	info := webhookInfo{
		eventType: e.Type,
		eventID:   e.ID,
		requestID: requestIDFrom(e.Request),
	}
	if e.Livemode != nil {
		info.livemode = "test"
		if *e.Livemode {
			info.livemode = "live"
		}
	}
	for _, k := range opRefFields {
		info.refs = appendRef(info.refs, jsonString(e.Data.Object[k]))
	}
	return info
}

// requestIDFrom decodes the event's `request` field: modern API versions send
// an object {"id": "req_…"}, very old ones a bare "req_…" string, and async
// events (billing cycles, test-clock cascades) null.
func requestIDFrom(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var obj struct {
		ID string `json:"id"`
	}
	if json.Unmarshal(raw, &obj) == nil {
		return obj.ID
	}
	return jsonString(raw)
}

// outboundSeedRefs collects the object ids an outbound call demonstrably
// touched: known-prefix tokens in its request URI plus top-level response-body
// reference fields. These seed the call's operation so later async events can
// be adopted into it.
func outboundSeedRefs(uri string, respBody []byte) []string {
	var refs []string
	for _, tok := range strings.FieldsFunc(uri, isNotIDRune) {
		for _, p := range opRefPrefixes {
			if strings.HasPrefix(tok, p) && len(tok) > len(p) {
				refs = appendRef(refs, tok)
				break
			}
		}
	}
	var top map[string]json.RawMessage
	if json.Unmarshal(respBody, &top) == nil {
		for _, k := range opRefFields {
			refs = appendRef(refs, jsonString(top[k]))
		}
	}
	return refs
}

func isNotIDRune(r rune) bool {
	return r != '_' &&
		(r < 'a' || r > 'z') && (r < 'A' || r > 'Z') && (r < '0' || r > '9')
}

// jsonString decodes raw as a JSON string, returning "" for null, absent, or
// non-string values (e.g. an expanded object where an id was expected).
func jsonString(raw json.RawMessage) string {
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	return ""
}

func appendRef(refs []string, s string) []string {
	if s == "" {
		return refs
	}
	for _, r := range refs {
		if r == s {
			return refs
		}
	}
	return append(refs, s)
}
