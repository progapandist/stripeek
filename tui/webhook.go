package tui

import "encoding/json"

// webhookMeta pulls the event type and primary object id from a forwarded Stripe
// event body. Returns zero values when the body isn't a recognisable event
// (including when truncation broke the JSON, which is fine — callers fall back
// to the request path).
func webhookMeta(body []byte) (eventType, objectID string) {
	var e struct {
		Type string `json:"type"`
		Data struct {
			Object struct {
				ID string `json:"id"`
			} `json:"object"`
		} `json:"data"`
	}
	if json.Unmarshal(body, &e) == nil {
		return e.Type, e.Data.Object.ID
	}
	return "", ""
}
