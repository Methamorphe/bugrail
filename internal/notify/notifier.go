package notify

import "context"

// NewIssueEvent carries the data passed to notifiers when a new issue is created.
type NewIssueEvent struct {
	IssueID     string
	Title       string
	Platform    string
	Environment string
	ProjectName string
	Level       string
	Culprit     string
	URL         string // full URL to the issue detail page, if base URL is configured
}

// Notifier sends an alert about a new issue.
type Notifier interface {
	Notify(ctx context.Context, evt NewIssueEvent) error
}
