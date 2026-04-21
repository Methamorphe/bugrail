package processor

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNormalizeEventUsesFingerprintWhenPresent(t *testing.T) {
	evt := Event{
		EventID:     "abc123",
		Platform:    "javascript",
		Environment: "production",
		Culprit:     "checkout",
		Fingerprint: []string{"checkout", "{{ default }}"},
		Exception: &ExceptionValues{Values: []ExceptionValue{
			{Type: "TypeError", Value: "Cannot read properties of undefined"},
		}},
	}

	normalized := normalizeEvent(evt)

	require.Equal(t, "abc123", normalized.EventID)
	require.Equal(t, "TypeError: Cannot read properties of undefined", normalized.IssueTitle)
	require.Equal(t, "checkout", normalized.Culprit)
	require.Equal(t, "javascript", normalized.Platform)
	require.Equal(t, "production", normalized.Environment)
	require.Equal(t, `["checkout","{{ default }}"]`, normalized.Fingerprint)
	require.Contains(t, normalized.GroupingKey, "fp:")
}

func TestNormalizeEventFallsBackToExceptionAndContext(t *testing.T) {
	evt := Event{
		Platform:    "python",
		Environment: "",
		Transaction: "jobs.sync",
		LogEntry: &LogEntry{
			Formatted: "sync failed",
		},
		Exception: &ExceptionValues{Values: []ExceptionValue{
			{Type: "ValueError", Value: "bad payload"},
		}},
	}

	normalized := normalizeEvent(evt)

	require.NotEmpty(t, normalized.EventID)
	require.Equal(t, "ValueError: bad payload", normalized.IssueTitle)
	require.Equal(t, "jobs.sync", normalized.Culprit)
	require.Equal(t, "production", normalized.Environment)
	require.Equal(t, "error", normalized.Level)
	require.Contains(t, normalized.GroupingKey, "fallback:")
}
