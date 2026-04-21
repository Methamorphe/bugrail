package web

import (
	"embed"
	"encoding/json"
	"html/template"
	"time"
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
		"parseBreadcrumbs": parseBreadcrumbs,
	}
	return template.New("pages").Funcs(funcMap).ParseFS(templateFS, "templates/*.html")
}
