package web

import (
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"strings"
	"time"

	"github.com/Methamorphe/bugrail/internal/storage"
)

//go:embed templates/*.html
var templateFS embed.FS

func loadTemplates() (*template.Template, error) {
	funcMap := template.FuncMap{
		"formatUnix": func(v int64) string {
			if v <= 0 {
				return "n/a"
			}
			return time.Unix(v, 0).Format("2006-01-02 15:04:05")
		},
		"prettyJSON": func(raw string) string {
			if raw == "" {
				return ""
			}
			var decoded any
			if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
				return raw
			}
			formatted, err := json.MarshalIndent(decoded, "", "  ")
			if err != nil {
				return raw
			}
			return string(formatted)
		},
		"parseStackFrames":  parseStackFrames,
		"parseEventContext": parseEventContext,
		"parseBreadcrumbs":  parseBreadcrumbs,
		"sparklineSVG":      sparklineSVG,
		"levelColor":        levelColor,
		"pct": func(count, total int64) int64 {
			if total == 0 {
				return 0
			}
			v := count * 100 / total
			if v < 1 && count > 0 {
				return 1
			}
			return v
		},
		"sub": func(a, b int) int { return a - b },
	}
	return template.New("pages").Funcs(funcMap).ParseFS(templateFS, "templates/*.html")
}

// sparklineSVG generates an inline SVG bar chart from daily event counts.
func sparklineSVG(data []storage.DayCount) template.HTML {
	const w, h, barGap = 560, 64, 2
	if len(data) == 0 {
		return template.HTML(fmt.Sprintf(`<svg width="%d" height="%d" xmlns="http://www.w3.org/2000/svg"></svg>`, w, h))
	}
	var maxCount int64
	for _, d := range data {
		if d.Count > maxCount {
			maxCount = d.Count
		}
	}
	if maxCount == 0 {
		maxCount = 1
	}
	n := len(data)
	barW := float64(w-barGap*(n-1)) / float64(n)
	var bars strings.Builder
	for i, d := range data {
		barH := float64(h) * float64(d.Count) / float64(maxCount)
		if barH < 1 {
			barH = 1
		}
		x := float64(i) * (barW + barGap)
		y := float64(h) - barH
		bars.WriteString(fmt.Sprintf(`<rect x="%.2f" y="%.2f" width="%.2f" height="%.2f" rx="1" fill="#10b981" opacity="0.7"/>`, x, y, barW, barH))
	}
	return template.HTML(fmt.Sprintf(
		`<svg width="%d" height="%d" xmlns="http://www.w3.org/2000/svg" class="w-full">%s</svg>`,
		w, h, bars.String(),
	))
}

// levelColor returns a Tailwind color pair for a given Sentry level.
func levelColor(level string) string {
	switch level {
	case "fatal":
		return "bg-rose-950/60 text-rose-300"
	case "error":
		return "bg-rose-900/40 text-rose-400"
	case "warning":
		return "bg-amber-950/60 text-amber-300"
	case "info":
		return "bg-sky-950/60 text-sky-300"
	default:
		return "bg-slate-800 text-slate-400"
	}
}
