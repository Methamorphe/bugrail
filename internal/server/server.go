package server

import (
	"log/slog"
	"net/http"

	"github.com/Methamorphe/bugrail/internal/auth"
	"github.com/Methamorphe/bugrail/internal/hub"
	"github.com/Methamorphe/bugrail/internal/ingest"
	"github.com/Methamorphe/bugrail/internal/notify"
	"github.com/Methamorphe/bugrail/internal/processor"
	"github.com/Methamorphe/bugrail/internal/ratelimit"
	"github.com/Methamorphe/bugrail/internal/storage"
	"github.com/Methamorphe/bugrail/internal/web"
)

// New constructs the Bugrail HTTP handler tree.
func New(store storage.Store, logger *slog.Logger, rateLimitPerProject int, baseURL string) (http.Handler, error) {
	h := hub.New()
	authService := auth.New(store)
	notifier := notify.FromEnv()
	processorService := processor.NewWithNotifier(store, notifier, logger, baseURL)
	limiter := ratelimit.New(rateLimitPerProject)
	ingestHandler := ingest.New(store, processorService, logger, h, limiter)
	webHandler, err := web.New(store, authService, logger, h)
	if err != nil {
		return nil, err
	}

	webMux := http.NewServeMux()
	webMux.HandleFunc("GET /", webHandler.HandleRoot)
	webMux.HandleFunc("GET /login", webHandler.HandleLogin)
	webMux.HandleFunc("POST /login", webHandler.HandleLogin)
	webMux.Handle("POST /logout", authService.RequireUser(http.HandlerFunc(webHandler.HandleLogout)))
	webMux.Handle("GET /issues", authService.RequireUser(http.HandlerFunc(webHandler.HandleIssues)))
	webMux.Handle("GET /issues/{id}", authService.RequireUser(http.HandlerFunc(webHandler.HandleIssue)))
	webMux.Handle("POST /issues/{id}/resolve", authService.RequireUser(http.HandlerFunc(webHandler.HandleIssueStatus)))
	webMux.Handle("POST /issues/{id}/ignore", authService.RequireUser(http.HandlerFunc(webHandler.HandleIssueStatus)))
	webMux.Handle("POST /issues/{id}/reopen", authService.RequireUser(http.HandlerFunc(webHandler.HandleIssueStatus)))
	webMux.Handle("GET /stream", authService.RequireUser(http.HandlerFunc(webHandler.HandleStream)))
	webMux.Handle("GET /attachments/{attachment_id}", authService.RequireUser(http.HandlerFunc(webHandler.HandleAttachment)))

	root := http.NewServeMux()
	root.HandleFunc("POST /api/{project_id}/envelope/", ingestHandler.HandleEnvelope)
	root.HandleFunc("POST /api/{project_id}/store/", ingestHandler.HandleStore)
	root.Handle("/", authService.SessionMiddleware(authService.CSRFMiddleware(webMux)))

	return root, nil
}
