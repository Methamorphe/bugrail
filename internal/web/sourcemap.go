package web

import (
	"path"
	"strings"

	"github.com/go-sourcemap/sourcemap"
)

// remapFrame applies a JavaScript source map to a minified stack frame.
// Returns the original frame unchanged if the source map cannot be applied.
func remapFrame(content string, frame StackFrame) StackFrame {
	m, err := sourcemap.Parse("", []byte(content))
	if err != nil {
		return frame
	}
	src, fn, line, col, ok := m.Source(frame.Lineno, frame.Colno)
	if !ok || src == "" {
		return frame
	}
	result := frame
	result.Filename = src
	result.AbsPath = src
	result.Lineno = line
	result.Colno = col
	if fn != "" {
		result.Function = fn
	}
	return result
}

// sourceMapFilenames returns candidate source map lookup keys for a frame.
// Tries the frame's abs_path and filename with and without ".map" suffix.
func sourceMapFilenames(frame StackFrame) []string {
	var candidates []string
	for _, raw := range []string{frame.AbsPath, frame.Filename} {
		if raw == "" {
			continue
		}
		base := path.Base(strings.SplitN(raw, "?", 2)[0])
		if base == "" || base == "." {
			continue
		}
		if strings.HasSuffix(base, ".map") {
			candidates = append(candidates, base)
		} else {
			candidates = append(candidates, base+".map")
		}
	}
	return dedupStrings(candidates)
}

func dedupStrings(ss []string) []string {
	seen := make(map[string]struct{}, len(ss))
	out := ss[:0]
	for _, s := range ss {
		if _, ok := seen[s]; !ok {
			seen[s] = struct{}{}
			out = append(out, s)
		}
	}
	return out
}
