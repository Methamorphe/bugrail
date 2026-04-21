package web

import "encoding/json"

// StackFrame holds the display data for a single Sentry stack frame.
type StackFrame struct {
	Filename    string
	AbsPath     string
	Module      string
	Function    string
	Lineno      int
	Colno       int
	InApp       bool
	ContextLine string
	PreContext  []string
	PostContext []string
}

type sentryPayload struct {
	Exception *struct {
		Values []struct {
			Stacktrace *struct {
				Frames []struct {
					Filename    string   `json:"filename"`
					AbsPath     string   `json:"abs_path"`
					Module      string   `json:"module"`
					Function    string   `json:"function"`
					Lineno      int      `json:"lineno"`
					Colno       int      `json:"colno"`
					InApp       bool     `json:"in_app"`
					ContextLine string   `json:"context_line"`
					PreContext  []string `json:"pre_context"`
					PostContext []string `json:"post_context"`
				} `json:"frames"`
			} `json:"stacktrace"`
		} `json:"values"`
	} `json:"exception"`
}

// parseStackFrames extracts stack frames from a raw Sentry event JSON payload.
// Frames are returned newest-first (reversed from Sentry's oldest-first storage).
// Returns nil if no stack trace is present.
func parseStackFrames(payload string) []StackFrame {
	if payload == "" {
		return nil
	}
	var p sentryPayload
	if err := json.Unmarshal([]byte(payload), &p); err != nil || p.Exception == nil {
		return nil
	}
	for _, v := range p.Exception.Values {
		if v.Stacktrace == nil || len(v.Stacktrace.Frames) == 0 {
			continue
		}
		src := v.Stacktrace.Frames
		out := make([]StackFrame, len(src))
		for i, f := range src {
			out[len(src)-1-i] = StackFrame{
				Filename:    f.Filename,
				AbsPath:     f.AbsPath,
				Module:      f.Module,
				Function:    f.Function,
				Lineno:      f.Lineno,
				Colno:       f.Colno,
				InApp:       f.InApp,
				ContextLine: f.ContextLine,
				PreContext:  f.PreContext,
				PostContext: f.PostContext,
			}
		}
		return out
	}
	return nil
}
