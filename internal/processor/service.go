package processor

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/Methamorphe/bugrail/internal/notify"
	"github.com/Methamorphe/bugrail/internal/storage"
)

type normalizedEvent struct {
	EventID        string
	GroupingKey    string
	IssueTitle     string
	Platform       string
	Environment    string
	Release        string
	Level          string
	Culprit        string
	ExceptionType  string
	ExceptionValue string
	Fingerprint    string
}

// Service applies Bugrail's first-pass normalization and grouping before storage.
type Service struct {
	store    storage.Store
	notifier *notify.Multi
	logger   *slog.Logger
	baseURL  string
}

// New creates a processor service.
func New(store storage.Store) Service {
	return Service{store: store}
}

// NewWithNotifier creates a processor service that fires notifications on new issues.
func NewWithNotifier(store storage.Store, notifier *notify.Multi, logger *slog.Logger, baseURL string) Service {
	return Service{store: store, notifier: notifier, logger: logger, baseURL: baseURL}
}

// Process normalizes an ingested Sentry event and persists it.
func (s Service) Process(ctx context.Context, project storage.ProjectRef, evt Event, rawPayload []byte, receivedAt int64) (storage.RecordProcessedEventResult, error) {
	normalized := normalizeEvent(evt)
	result, err := s.store.RecordProcessedEvent(ctx, storage.RecordProcessedEventParams{
		ProjectID:      project.ProjectID,
		EventID:        normalized.EventID,
		GroupingKey:    normalized.GroupingKey,
		IssueTitle:     normalized.IssueTitle,
		Platform:       normalized.Platform,
		Environment:    normalized.Environment,
		Release:        normalized.Release,
		Level:          normalized.Level,
		Culprit:        normalized.Culprit,
		ExceptionType:  normalized.ExceptionType,
		ExceptionValue: normalized.ExceptionValue,
		Fingerprint:    normalized.Fingerprint,
		Payload:        string(rawPayload),
		ReceivedAt:     receivedAt,
	})
	if err != nil {
		return storage.RecordProcessedEventResult{}, fmt.Errorf("record processed event: %w", err)
	}

	if result.NewIssue && s.notifier != nil && s.notifier.Enabled() {
		issueURL := ""
		if s.baseURL != "" {
			issueURL = s.baseURL + "/issues/" + result.IssueID
		}
		notifyErr := s.notifier.Notify(ctx, notify.NewIssueEvent{
			IssueID:     result.IssueID,
			Title:       normalized.IssueTitle,
			Platform:    normalized.Platform,
			Environment: normalized.Environment,
			ProjectName: project.ProjectName,
			Level:       normalized.Level,
			Culprit:     normalized.Culprit,
			URL:         issueURL,
		})
		if notifyErr != nil && s.logger != nil {
			s.logger.WarnContext(ctx, "notify new issue failed", "err", notifyErr, "issue_id", result.IssueID)
		}
	}

	return result, nil
}
