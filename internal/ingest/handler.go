package ingest

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/Methamorphe/bugrail/internal/hub"
	"github.com/Methamorphe/bugrail/internal/processor"
	"github.com/Methamorphe/bugrail/internal/ratelimit"
	"github.com/Methamorphe/bugrail/internal/storage"
)

// Handler exposes Sentry-compatible ingestion endpoints.
type Handler struct {
	store     storage.Store
	processor processor.Service
	logger    *slog.Logger
	hub       *hub.Hub
	limiter   *ratelimit.Limiter
}

// New creates a new ingestion handler set.
func New(store storage.Store, processor processor.Service, logger *slog.Logger, h *hub.Hub, limiter *ratelimit.Limiter) Handler {
	return Handler{
		store:     store,
		processor: processor,
		logger:    logger,
		hub:       h,
		limiter:   limiter,
	}
}

// HandleEnvelope ingests a Sentry envelope and persists event items.
func (h Handler) HandleEnvelope(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("project_id")
	if projectID == "" {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	body, err := readRequestBody(r)
	if err != nil {
		h.logger.ErrorContext(r.Context(), "read envelope body failed", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	publicKey := extractPublicKey(r)
	if publicKey == "" {
		publicKey = extractPublicKeyFromEnvelopeHeader(body)
	}
	if publicKey == "" {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	project, err := h.store.AuthenticateProjectKey(r.Context(), projectID, publicKey)
	if err != nil {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	if !h.limiter.Allow(project.ProjectID) {
		writeRateLimitResponse(w)
		return
	}

	items, err := parseEnvelope(body)
	if err != nil {
		h.logger.WarnContext(r.Context(), "parse envelope failed", "err", err)
		writeIngestResponse(w, "")
		return
	}

	lastEventID := ""
	lastRelease := ""
	for _, item := range items {
		switch item.Type {
		case "event":
			var evt processor.Event
			if err := json.Unmarshal(item.Payload, &evt); err != nil {
				h.logger.WarnContext(r.Context(), "decode envelope event failed", "err", err)
				continue
			}
			result, err := h.processor.Process(r.Context(), project, evt, item.Payload, time.Now().Unix())
			if err != nil {
				h.logger.ErrorContext(r.Context(), "process envelope event failed", "err", err, "project_id", project.ProjectID)
				http.Error(w, "internal error", http.StatusInternalServerError)
				return
			}
			if !result.Duplicate {
				h.hub.Notify()
			}
			lastEventID = result.EventID
			lastRelease = evt.Release
		case "attachment":
			if len(item.Payload) == 0 {
				continue
			}
			id, idErr := randomID()
			if idErr != nil {
				h.logger.WarnContext(r.Context(), "generate attachment id failed", "err", idErr)
				continue
			}
			// Source map attachments: store separately for frame remapping.
			if lastRelease != "" && isSourceMap(item.Filename, item.ContentType) {
				if err := h.store.StoreSourceMap(r.Context(), storage.SourceMapParams{
					ID:        id,
					ProjectID: project.ProjectID,
					Release:   lastRelease,
					Filename:  item.Filename,
					Content:   string(item.Payload),
					CreatedAt: time.Now().Unix(),
				}); err != nil {
					h.logger.WarnContext(r.Context(), "store source map failed", "err", err)
				}
				continue
			}
			if lastEventID == "" {
				continue
			}
			if err := h.store.StoreAttachment(r.Context(), storage.AttachmentParams{
				ID:          id,
				EventID:     lastEventID,
				ProjectID:   project.ProjectID,
				Filename:    item.Filename,
				ContentType: item.ContentType,
				Data:        item.Payload,
				CreatedAt:   time.Now().Unix(),
			}); err != nil {
				h.logger.WarnContext(r.Context(), "store attachment failed", "err", err)
			}
		}
	}

	writeIngestResponse(w, lastEventID)
}

// HandleStore ingests the legacy store payload and normalizes it to the same event pipeline.
func (h Handler) HandleStore(w http.ResponseWriter, r *http.Request) {
	project, ok := h.authenticateProject(w, r)
	if !ok {
		return
	}

	if !h.limiter.Allow(project.ProjectID) {
		writeRateLimitResponse(w)
		return
	}

	body, err := readRequestBody(r)
	if err != nil {
		h.logger.ErrorContext(r.Context(), "read store body failed", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	var evt processor.Event
	if err := json.Unmarshal(body, &evt); err != nil {
		h.logger.WarnContext(r.Context(), "decode store event failed", "err", err)
		writeIngestResponse(w, "")
		return
	}

	result, err := h.processor.Process(r.Context(), project, evt, body, time.Now().Unix())
	if err != nil {
		h.logger.ErrorContext(r.Context(), "process store event failed", "err", err, "project_id", project.ProjectID)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if !result.Duplicate {
		h.hub.Notify()
	}

	writeIngestResponse(w, result.EventID)
}

func (h Handler) authenticateProject(w http.ResponseWriter, r *http.Request) (storage.ProjectRef, bool) {
	projectID := r.PathValue("project_id")
	publicKey := extractPublicKey(r)
	if projectID == "" || publicKey == "" {
		http.Error(w, "forbidden", http.StatusForbidden)
		return storage.ProjectRef{}, false
	}

	project, err := h.store.AuthenticateProjectKey(r.Context(), projectID, publicKey)
	if err != nil {
		http.Error(w, "forbidden", http.StatusForbidden)
		return storage.ProjectRef{}, false
	}
	return project, true
}

func isSourceMap(filename, contentType string) bool {
	return strings.HasSuffix(filename, ".map") &&
		strings.Contains(contentType, "json")
}

func randomID() (string, error) {
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func writeRateLimitResponse(w http.ResponseWriter) {
	// X-Sentry-Rate-Limits: retry_after:categories:scope:reason_code
	w.Header().Set("X-Sentry-Rate-Limits", "60:default:project:rate_limit")
	w.Header().Set("Retry-After", "60")
	http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
}

func writeIngestResponse(w http.ResponseWriter, eventID string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if eventID == "" {
		_, _ = w.Write([]byte(`{}`))
		return
	}
	_, _ = w.Write([]byte(`{"id":"` + eventID + `"}`))
}
