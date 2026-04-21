package processor

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
)

func normalizeEvent(evt Event) normalizedEvent {
	exType, exValue := primaryException(evt)
	issueTitle := issueTitle(evt, exType, exValue)
	culprit := firstNonEmpty(evt.Culprit, evt.Transaction, issueTitle)
	platform := firstNonEmpty(evt.Platform, "other")
	environment := firstNonEmpty(evt.Environment, "production")
	level := firstNonEmpty(evt.Level, "error")

	return normalizedEvent{
		EventID:        ensureEventID(evt.EventID),
		GroupingKey:    buildGroupingKey(evt, exType, exValue, culprit, platform, issueTitle),
		IssueTitle:     issueTitle,
		Platform:       platform,
		Environment:    environment,
		Release:        evt.Release,
		Level:          level,
		Culprit:        culprit,
		ExceptionType:  exType,
		ExceptionValue: exValue,
		Fingerprint:    encodeFingerprint(evt.Fingerprint),
	}
}

func buildGroupingKey(evt Event, exType, exValue, culprit, platform, issueTitle string) string {
	if len(evt.Fingerprint) > 0 {
		return "fp:" + hashParts(evt.Fingerprint...)
	}
	return "fallback:" + hashParts(exType, exValue, culprit, platform, issueTitle)
}

func primaryException(evt Event) (string, string) {
	if evt.Exception == nil || len(evt.Exception.Values) == 0 {
		return "", ""
	}
	return strings.TrimSpace(evt.Exception.Values[0].Type), strings.TrimSpace(evt.Exception.Values[0].Value)
}

func issueTitle(evt Event, exType, exValue string) string {
	switch {
	case exType != "" && exValue != "":
		return exType + ": " + exValue
	case exType != "":
		return exType
	}

	if evt.LogEntry != nil {
		if text := firstNonEmpty(evt.LogEntry.Formatted, evt.LogEntry.Message); text != "" {
			return text
		}
	}
	if text := strings.TrimSpace(evt.Message); text != "" {
		return text
	}
	if text := firstNonEmpty(evt.Culprit, evt.Transaction); text != "" {
		return text
	}
	return "Unhandled event"
}

func ensureEventID(id string) string {
	id = strings.TrimSpace(id)
	if id != "" {
		return id
	}
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return hashParts("generated-event-id")
	}
	return hex.EncodeToString(buf)
}

func encodeFingerprint(parts []string) string {
	if len(parts) == 0 {
		return ""
	}
	buf, err := json.Marshal(parts)
	if err != nil {
		return ""
	}
	return string(buf)
}

func hashParts(parts ...string) string {
	h := sha256.New()
	for _, part := range parts {
		h.Write([]byte(strings.TrimSpace(part)))
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}
