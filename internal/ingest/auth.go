package ingest

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
)

func extractPublicKey(r *http.Request) string {
	if key := parseSentryAuth(r.Header.Get("X-Sentry-Auth")); key != "" {
		return key
	}
	if key := parseSentryAuth(r.Header.Get("Authorization")); key != "" {
		return key
	}
	if key := strings.TrimSpace(r.URL.Query().Get("sentry_key")); key != "" {
		return key
	}
	return ""
}

func extractPublicKeyFromEnvelopeHeader(body []byte) string {
	idx := bytes.IndexByte(body, '\n')
	line := body
	if idx >= 0 {
		line = body[:idx]
	}
	var header struct {
		DSN string `json:"dsn"`
	}
	if err := json.Unmarshal(line, &header); err != nil || header.DSN == "" {
		return ""
	}
	u, err := url.Parse(header.DSN)
	if err != nil || u.User == nil {
		return ""
	}
	return u.User.Username()
}

func parseSentryAuth(header string) string {
	header = strings.TrimSpace(header)
	if header == "" {
		return ""
	}

	for _, part := range strings.Split(header, ",") {
		part = strings.TrimSpace(part)
		part = strings.TrimPrefix(part, "Sentry ")
		part = strings.TrimPrefix(part, "Bearer ")
		if !strings.HasPrefix(part, "sentry_key=") {
			continue
		}
		value := strings.TrimPrefix(part, "sentry_key=")
		return strings.Trim(strings.TrimSpace(value), `"`)
	}
	return ""
}
