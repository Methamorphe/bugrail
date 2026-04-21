package web

import "encoding/json"

// Breadcrumb holds display data for a single Sentry breadcrumb entry.
type Breadcrumb struct {
	Timestamp string
	Type      string
	Category  string
	Message   string
	Level     string
	Data      map[string]any
}

type rawBreadcrumbsPayload struct {
	Breadcrumbs *struct {
		Values []struct {
			Timestamp string         `json:"timestamp"`
			Type      string         `json:"type"`
			Category  string         `json:"category"`
			Message   string         `json:"message"`
			Level     string         `json:"level"`
			Data      map[string]any `json:"data"`
		} `json:"values"`
	} `json:"breadcrumbs"`
}

const maxBreadcrumbs = 20

// parseBreadcrumbs extracts the last 20 breadcrumbs from a Sentry event payload,
// returned newest-first. Returns nil if no breadcrumbs are present.
func parseBreadcrumbs(payload string) []Breadcrumb {
	if payload == "" {
		return nil
	}
	var p rawBreadcrumbsPayload
	if err := json.Unmarshal([]byte(payload), &p); err != nil || p.Breadcrumbs == nil {
		return nil
	}
	src := p.Breadcrumbs.Values
	if len(src) == 0 {
		return nil
	}

	// Take up to maxBreadcrumbs from the end (most recent).
	start := 0
	if len(src) > maxBreadcrumbs {
		start = len(src) - maxBreadcrumbs
	}
	src = src[start:]

	// Reverse: display newest first.
	out := make([]Breadcrumb, len(src))
	for i, b := range src {
		level := b.Level
		if level == "" {
			level = "info"
		}
		out[len(src)-1-i] = Breadcrumb{
			Timestamp: b.Timestamp,
			Type:      b.Type,
			Category:  b.Category,
			Message:   b.Message,
			Level:     level,
			Data:      b.Data,
		}
	}
	return out
}
