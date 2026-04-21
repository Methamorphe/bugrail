package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// WebhookNotifier POSTs a JSON payload to a configured URL.
type WebhookNotifier struct {
	url    string
	client *http.Client
}

// NewWebhook creates a WebhookNotifier that posts to the given URL.
func NewWebhook(url string) *WebhookNotifier {
	return &WebhookNotifier{url: url, client: &http.Client{}}
}

func (w *WebhookNotifier) Notify(ctx context.Context, evt NewIssueEvent) error {
	payload, err := json.Marshal(evt)
	if err != nil {
		return fmt.Errorf("webhook: marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, w.url, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("webhook: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Bugrail/1")

	resp, err := w.client.Do(req)
	if err != nil {
		return fmt.Errorf("webhook: send: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("webhook: unexpected status %d", resp.StatusCode)
	}
	return nil
}
