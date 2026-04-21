package web

import (
	"context"
	"errors"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Methamorphe/bugrail/internal/auth"
	"github.com/Methamorphe/bugrail/internal/hub"
	"github.com/Methamorphe/bugrail/internal/storage"
)

// eventView wraps a storage event with pre-parsed (and optionally source-map-remapped) frames.
type eventView struct {
	storage.EventRecord
	Frames []StackFrame
}

type pageData struct {
	Title        string
	ActivePage   string
	CSRFField    string
	CSRFToken    string
	User         *auth.User
	Error        string
	Issues       []storage.IssueSummary
	Issue        storage.IssueDetail
	Events       []eventView
	ProjectDSN   string
	Filter       storage.ListIssuesFilter
	NextCursor   int64
	Environments []string
	Stats        storage.DashboardStats
	EventVolume  []storage.DayCount
	Releases     []storage.ReleaseStat
}

// Handler renders the HTML application.
type Handler struct {
	store     storage.Store
	auth      auth.Service
	logger    *slog.Logger
	templates *template.Template
	hub       *hub.Hub
}

// New creates the web handler set.
func New(store storage.Store, authService auth.Service, logger *slog.Logger, h *hub.Hub) (Handler, error) {
	templates, err := loadTemplates()
	if err != nil {
		return Handler{}, fmt.Errorf("load templates: %w", err)
	}
	return Handler{
		store:     store,
		auth:      authService,
		logger:    logger,
		templates: templates,
		hub:       h,
	}, nil
}

// HandleRoot renders the dashboard for authenticated users or redirects to login.
func (h Handler) HandleRoot(w http.ResponseWriter, r *http.Request) {
	if _, ok := auth.CurrentUser(r.Context()); !ok {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	h.HandleDashboard(w, r)
}

// HandleDashboard renders the overview page with stats, chart, and top issues.
func (h Handler) HandleDashboard(w http.ResponseWriter, r *http.Request) {
	stats, err := h.store.GetDashboardStats(r.Context())
	if err != nil {
		h.logger.ErrorContext(r.Context(), "get dashboard stats failed", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	volume, err := h.store.GetEventVolumeByDay(r.Context(), 14)
	if err != nil {
		h.logger.WarnContext(r.Context(), "get event volume failed", "err", err)
	}
	topIssues, err := h.store.ListIssues(r.Context(), storage.ListIssuesFilter{Status: "open"}, 5)
	if err != nil {
		h.logger.WarnContext(r.Context(), "get top issues failed", "err", err)
	}
	h.render(w, r, "dashboard.html", pageData{
		Title:       "Dashboard",
		ActivePage:  "dashboard",
		User:        currentUser(r),
		CSRFField:   auth.CSRFFieldName(),
		CSRFToken:   auth.CSRFToken(r.Context()),
		Stats:       stats,
		EventVolume: volume,
		Issues:      topIssues,
	})
}

// HandleLogin renders and processes the login form.
func (h Handler) HandleLogin(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.render(w, r, "login.html", pageData{
			Title:     "Login",
			CSRFField: auth.CSRFFieldName(),
			CSRFToken: auth.CSRFToken(r.Context()),
		})
	case http.MethodPost:
		if err := r.ParseForm(); err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		_, err := h.auth.Login(r.Context(), w, r, r.FormValue("email"), r.FormValue("password"))
		if err != nil {
			status := http.StatusUnauthorized
			if !errors.Is(err, storage.ErrNotFound) {
				status = http.StatusInternalServerError
				h.logger.ErrorContext(r.Context(), "login failed", "err", err)
			}
			w.WriteHeader(status)
			h.render(w, r, "login.html", pageData{
				Title:     "Login",
				CSRFField: auth.CSRFFieldName(),
				CSRFToken: auth.CSRFToken(r.Context()),
				Error:     "Invalid email or password.",
			})
			return
		}
		http.Redirect(w, r, "/", http.StatusSeeOther)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// HandleLogout logs the current user out.
func (h Handler) HandleLogout(w http.ResponseWriter, r *http.Request) {
	if err := h.auth.Logout(r.Context(), w, r); err != nil {
		h.logger.ErrorContext(r.Context(), "logout failed", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

const issuesPageSize = 50

// HandleIssues renders the issues list with optional filtering and cursor pagination.
func (h Handler) HandleIssues(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	filter := storage.ListIssuesFilter{
		Status:      q.Get("status"),
		Platform:    q.Get("platform"),
		Environment: q.Get("env"),
		Level:       q.Get("level"),
		Search:      q.Get("q"),
	}
	if c := q.Get("cursor"); c != "" {
		if v, err := strconv.ParseInt(c, 10, 64); err == nil {
			filter.Cursor = v
		}
	}

	issues, err := h.store.ListIssues(r.Context(), filter, issuesPageSize+1)
	if err != nil {
		h.logger.ErrorContext(r.Context(), "list issues failed", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	envs, err := h.store.ListDistinctEnvironments(r.Context())
	if err != nil {
		h.logger.WarnContext(r.Context(), "list environments failed", "err", err)
	}

	var nextCursor int64
	if len(issues) > issuesPageSize {
		issues = issues[:issuesPageSize]
		nextCursor = issues[issuesPageSize-1].LastSeenAt
	}

	h.render(w, r, "issues.html", pageData{
		Title:        "Issues",
		ActivePage:   "issues",
		CSRFField:    auth.CSRFFieldName(),
		CSRFToken:    auth.CSRFToken(r.Context()),
		User:         currentUser(r),
		Issues:       issues,
		Filter:       filter,
		NextCursor:   nextCursor,
		Environments: envs,
	})
}

// HandleIssue renders a single issue and its recent events.
func (h Handler) HandleIssue(w http.ResponseWriter, r *http.Request) {
	issueID := r.PathValue("id")
	issue, err := h.store.GetIssue(r.Context(), issueID)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			http.NotFound(w, r)
			return
		}
		h.logger.ErrorContext(r.Context(), "get issue failed", "err", err, "issue_id", issueID)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	rawEvents, err := h.store.ListEventsByIssue(r.Context(), issueID, 25)
	if err != nil {
		h.logger.ErrorContext(r.Context(), "list events by issue failed", "err", err, "issue_id", issueID)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	views := make([]eventView, 0, len(rawEvents))
	for _, ev := range rawEvents {
		attachments, aErr := h.store.ListAttachmentsByEventID(r.Context(), ev.EventID)
		if aErr != nil {
			h.logger.WarnContext(r.Context(), "list attachments failed", "err", aErr, "event_id", ev.EventID)
		} else {
			ev.Attachments = attachments
		}

		frames := parseStackFrames(ev.Payload)
		if len(frames) > 0 && ev.Release != "" {
			frames = h.remapFrames(r.Context(), issue.ProjectID, ev.Release, frames)
		}
		views = append(views, eventView{EventRecord: ev, Frames: frames})
	}

	h.render(w, r, "issue.html", pageData{
		Title:     issue.Title,
		CSRFField: auth.CSRFFieldName(),
		CSRFToken: auth.CSRFToken(r.Context()),
		User:      currentUser(r),
		Issue:     issue,
		Events:    views,
	})
}

func (h Handler) render(w http.ResponseWriter, r *http.Request, name string, data pageData) {
	if data.User == nil {
		data.User = currentUser(r)
	}
	if err := h.templates.ExecuteTemplate(w, name, data); err != nil {
		h.logger.ErrorContext(r.Context(), "render template failed", "err", err, "template", name)
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}

// HandleIssueStatus processes resolve / ignore / reopen actions on an issue.
func (h Handler) HandleIssueStatus(w http.ResponseWriter, r *http.Request) {
	issueID := r.PathValue("id")

	parts := strings.Split(strings.TrimSuffix(r.URL.Path, "/"), "/")
	action := parts[len(parts)-1]

	statusMap := map[string]string{
		"resolve": "resolved",
		"ignore":  "ignored",
		"reopen":  "open",
	}
	status, ok := statusMap[action]
	if !ok {
		http.NotFound(w, r)
		return
	}

	if err := h.store.UpdateIssueStatus(r.Context(), issueID, status, time.Now().Unix()); err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			http.NotFound(w, r)
			return
		}
		h.logger.ErrorContext(r.Context(), "update issue status failed", "err", err, "issue_id", issueID)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	issue, err := h.store.GetIssue(r.Context(), issueID)
	if err != nil {
		h.logger.ErrorContext(r.Context(), "get issue after status update failed", "err", err, "issue_id", issueID)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	if r.Header.Get("HX-Request") == "" {
		http.Redirect(w, r, "/issues/"+issueID, http.StatusSeeOther)
		return
	}

	if r.URL.Query().Get("ctx") == "detail" {
		h.renderPartial(w, "issue-actions", issue)
		return
	}
	h.renderPartial(w, "issue-row", issue.IssueSummary)
}

// HandleReleases renders the releases overview page.
func (h Handler) HandleReleases(w http.ResponseWriter, r *http.Request) {
	releases, err := h.store.ListReleaseStats(r.Context())
	if err != nil {
		h.logger.ErrorContext(r.Context(), "list release stats failed", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	h.render(w, r, "releases.html", pageData{
		Title:      "Releases",
		ActivePage: "releases",
		CSRFField:  auth.CSRFFieldName(),
		CSRFToken:  auth.CSRFToken(r.Context()),
		User:       currentUser(r),
		Releases:   releases,
	})
}

// HandleStream streams SSE notifications to the browser whenever issues change.
func (h Handler) HandleStream(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")

	notify, unsub := h.hub.Subscribe()
	defer unsub()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-notify:
			fmt.Fprintf(w, "data: refresh\n\n")
			flusher.Flush()
		}
	}
}

func (h Handler) renderPartial(w http.ResponseWriter, name string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := h.templates.ExecuteTemplate(w, name, data); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}

// HandleAttachment streams a stored attachment file to the browser.
func (h Handler) HandleAttachment(w http.ResponseWriter, r *http.Request) {
	attachmentID := r.PathValue("attachment_id")
	data, contentType, err := h.store.GetAttachmentData(r.Context(), attachmentID)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			http.NotFound(w, r)
			return
		}
		h.logger.ErrorContext(r.Context(), "get attachment failed", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", "attachment")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func currentUser(r *http.Request) *auth.User {
	user, ok := auth.CurrentUser(r.Context())
	if !ok {
		return nil
	}
	return &user
}

// remapFrames attempts to apply stored source maps to a slice of stack frames.
// Frames for which no source map is found are returned unchanged.
func (h Handler) remapFrames(ctx context.Context, projectID, release string, frames []StackFrame) []StackFrame {
	out := make([]StackFrame, len(frames))
	copy(out, frames)
	for i, f := range out {
		for _, candidate := range sourceMapFilenames(f) {
			content, err := h.store.GetSourceMap(ctx, projectID, release, candidate)
			if err != nil {
				continue
			}
			out[i] = remapFrame(content, f)
			break
		}
	}
	return out
}
