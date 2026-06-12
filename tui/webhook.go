package tui

import "encoding/json"

// webhookInfo is presentation metadata derived once from a forwarded Stripe
// event body, so rows and the inspector header never re-parse JSON per frame.
type webhookInfo struct {
	eventType string // body `type`, e.g. "customer.subscription.created"
	eventID   string // body `id`, evt_…
	livemode  string // "live" or "test" (modeBadge vocabulary); "" when unknown
}

// webhookMeta pulls event metadata from a forwarded Stripe event body. Returns
// the zero value when the body isn't a recognisable event (including when
// truncation broke the JSON, which is fine — callers fall back to the request
// path).
func webhookMeta(body []byte) webhookInfo {
	var e struct {
		ID       string `json:"id"`
		Type     string `json:"type"`
		Livemode *bool  `json:"livemode"`
	}
	if json.Unmarshal(body, &e) != nil {
		return webhookInfo{}
	}
	info := webhookInfo{eventType: e.Type, eventID: e.ID}
	if e.Livemode != nil {
		info.livemode = "test"
		if *e.Livemode {
			info.livemode = "live"
		}
	}
	return info
}
