package web

import "encoding/json"

// EventContext holds structured display data extracted from a Sentry event payload.
type EventContext struct {
	Tags    []KVPair
	User    *UserCtx
	Request *RequestCtx
}

// KVPair is a display-ready key/value pair (tags, headers).
type KVPair struct {
	Key   string
	Value string
}

// UserCtx holds Sentry user context fields.
type UserCtx struct {
	ID        string
	Email     string
	Username  string
	IPAddress string
}

// RequestCtx holds Sentry HTTP request context fields.
type RequestCtx struct {
	URL         string
	Method      string
	QueryString string
	Headers     []KVPair
}

// rawEventContext is used only for JSON unmarshalling.
type rawEventContext struct {
	Tags    json.RawMessage `json:"tags"`
	User    *struct {
		ID        string `json:"id"`
		Email     string `json:"email"`
		Username  string `json:"username"`
		IPAddress string `json:"ip_address"`
	} `json:"user"`
	Request *struct {
		URL         string          `json:"url"`
		Method      string          `json:"method"`
		QueryString string          `json:"query_string"`
		Headers     json.RawMessage `json:"headers"`
	} `json:"request"`
}

// parseEventContext extracts tags, user, and request context from a Sentry payload.
// Returns nil if the payload is empty or unparseable.
func parseEventContext(payload string) *EventContext {
	if payload == "" {
		return nil
	}
	var raw rawEventContext
	if err := json.Unmarshal([]byte(payload), &raw); err != nil {
		return nil
	}

	ctx := &EventContext{}
	ctx.Tags = parseKVField(raw.Tags)

	if raw.User != nil {
		ctx.User = &UserCtx{
			ID:        raw.User.ID,
			Email:     raw.User.Email,
			Username:  raw.User.Username,
			IPAddress: raw.User.IPAddress,
		}
		if ctx.User.ID == "" && ctx.User.Email == "" && ctx.User.Username == "" && ctx.User.IPAddress == "" {
			ctx.User = nil
		}
	}

	if raw.Request != nil {
		ctx.Request = &RequestCtx{
			URL:         raw.Request.URL,
			Method:      raw.Request.Method,
			QueryString: raw.Request.QueryString,
			Headers:     parseKVField(raw.Request.Headers),
		}
		if ctx.Request.URL == "" && ctx.Request.Method == "" && len(ctx.Request.Headers) == 0 {
			ctx.Request = nil
		}
	}

	if len(ctx.Tags) == 0 && ctx.User == nil && ctx.Request == nil {
		return nil
	}
	return ctx
}

// parseKVField parses a JSON field that is either an array of [key, value] pairs
// or a {"key": "value"} object into a sorted []KVPair slice.
func parseKVField(raw json.RawMessage) []KVPair {
	if len(raw) == 0 {
		return nil
	}

	// Try array of pairs: [["key","value"], ...]
	var pairs [][]json.RawMessage
	if json.Unmarshal(raw, &pairs) == nil && len(pairs) > 0 {
		out := make([]KVPair, 0, len(pairs))
		for _, p := range pairs {
			if len(p) != 2 {
				continue
			}
			var k, v string
			if json.Unmarshal(p[0], &k) != nil || json.Unmarshal(p[1], &v) != nil {
				continue
			}
			out = append(out, KVPair{Key: k, Value: v})
		}
		if len(out) > 0 {
			return out
		}
	}

	// Fall back to object: {"key": "value"}
	var obj map[string]string
	if json.Unmarshal(raw, &obj) == nil {
		out := make([]KVPair, 0, len(obj))
		for k, v := range obj {
			out = append(out, KVPair{Key: k, Value: v})
		}
		return out
	}

	return nil
}
