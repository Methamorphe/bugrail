package notify

import (
	"context"
	"fmt"
	"net/http"
	"strings"
)

// NtfyNotifier sends a push notification via ntfy.sh or a self-hosted ntfy server.
type NtfyNotifier struct {
	url    string
	client *http.Client
}

// NewNtfy creates an NtfyNotifier that posts to the given topic URL.
func NewNtfy(topicURL string) *NtfyNotifier {
	return &NtfyNotifier{url: topicURL, client: &http.Client{}}
}

func (n *NtfyNotifier) Notify(ctx context.Context, evt NewIssueEvent) error {
	body := evt.Title
	if evt.Culprit != "" {
		body += "\n" + evt.Culprit
	}
	if evt.URL != "" {
		body += "\n" + evt.URL
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, n.url, strings.NewReader(body))
	if err != nil {
		return fmt.Errorf("ntfy: build request: %w", err)
	}
	req.Header.Set("Title", fmt.Sprintf("[%s] New issue: %s", evt.ProjectName, evt.Platform))
	req.Header.Set("Tags", "bug")
	if evt.Level == "fatal" || evt.Level == "error" {
		req.Header.Set("Priority", "high")
	}

	resp, err := n.client.Do(req)
	if err != nil {
		return fmt.Errorf("ntfy: send: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("ntfy: unexpected status %d", resp.StatusCode)
	}
	return nil
}
