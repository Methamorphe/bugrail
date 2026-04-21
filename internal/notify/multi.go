package notify

import (
	"context"
	"errors"
	"os"
	"strings"
)

// Multi fans out a notification to all registered notifiers.
// Errors are joined; a failure in one notifier does not stop the others.
type Multi struct {
	notifiers []Notifier
}

// FromEnv builds a Multi from environment variables:
//   - BUGRAIL_NTFY_URL — ntfy topic URL
//   - BUGRAIL_WEBHOOK_URL — comma-separated webhook URLs
func FromEnv() *Multi {
	var nn []Notifier

	if u := strings.TrimSpace(os.Getenv("BUGRAIL_NTFY_URL")); u != "" {
		nn = append(nn, NewNtfy(u))
	}

	if raw := strings.TrimSpace(os.Getenv("BUGRAIL_WEBHOOK_URL")); raw != "" {
		for _, u := range strings.Split(raw, ",") {
			u = strings.TrimSpace(u)
			if u != "" {
				nn = append(nn, NewWebhook(u))
			}
		}
	}

	return &Multi{notifiers: nn}
}

// Enabled reports whether at least one notifier is configured.
func (m *Multi) Enabled() bool { return len(m.notifiers) > 0 }

// Notify delivers the event to all notifiers, collecting errors.
func (m *Multi) Notify(ctx context.Context, evt NewIssueEvent) error {
	var errs []error
	for _, n := range m.notifiers {
		if err := n.Notify(ctx, evt); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}
