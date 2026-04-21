package ingest

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Methamorphe/bugrail/internal/hub"
	"github.com/Methamorphe/bugrail/internal/processor"
	"github.com/Methamorphe/bugrail/internal/ratelimit"
	"github.com/Methamorphe/bugrail/internal/storage"
)

func TestHandleStoreNormalizesEventAndReturnsOK(t *testing.T) {
	store := &fakeStore{
		project: storage.ProjectRef{
			ProjectID:   "1",
			ProjectName: "Bugrail",
			PublicKey:   "public123",
		},
	}
	handler := New(store, processor.New(store), slog.New(slog.NewTextHandler(io.Discard, nil)), hub.New(), ratelimit.New(1000))

	body := []byte(`{"event_id":"evt-1","platform":"javascript","message":"checkout failed","culprit":"checkout","exception":{"values":[{"type":"TypeError","value":"boom"}]}}`)
	req := httptest.NewRequest(http.MethodPost, "/api/1/store/?sentry_key=public123", bytes.NewReader(body))
	req.SetPathValue("project_id", "1")
	rec := httptest.NewRecorder()

	handler.HandleStore(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.JSONEq(t, `{"id":"evt-1"}`, rec.Body.String())
	require.NotNil(t, store.lastRecord)
	require.Equal(t, "1", store.lastRecord.ProjectID)
	require.Equal(t, "evt-1", store.lastRecord.EventID)
	require.Equal(t, "TypeError: boom", store.lastRecord.IssueTitle)
	require.Equal(t, "checkout", store.lastRecord.Culprit)
}

func TestHandleEnvelopeIgnoresUnknownItemsAndParsesEvent(t *testing.T) {
	store := &fakeStore{
		project: storage.ProjectRef{
			ProjectID:   "1",
			ProjectName: "Bugrail",
			PublicKey:   "public123",
		},
	}
	handler := New(store, processor.New(store), slog.New(slog.NewTextHandler(io.Discard, nil)), hub.New(), ratelimit.New(1000))

	envelope := []byte("{\"event_id\":\"evt-2\"}\n{\"type\":\"session\"}\n{}\n{\"type\":\"event\"}\n{\"event_id\":\"evt-2\",\"platform\":\"python\",\"message\":\"job failed\"}\n")
	req := httptest.NewRequest(http.MethodPost, "/api/1/envelope/?sentry_key=public123", bytes.NewReader(envelope))
	req.SetPathValue("project_id", "1")
	rec := httptest.NewRecorder()

	handler.HandleEnvelope(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.JSONEq(t, `{"id":"evt-2"}`, rec.Body.String())
	require.NotNil(t, store.lastRecord)
	require.Equal(t, "evt-2", store.lastRecord.EventID)
	require.Equal(t, "job failed", store.lastRecord.IssueTitle)
}

func TestParseEnvelopeGoldenFiles(t *testing.T) {
	t.Parallel()

	files := []string{
		"../../testdata/envelopes/js.envelope",
		"../../testdata/envelopes/php.envelope",
		"../../testdata/envelopes/python.envelope",
	}

	for _, path := range files {
		path := path
		t.Run(path, func(t *testing.T) {
			t.Parallel()
			body, err := os.ReadFile(path)
			require.NoError(t, err)

			items, err := parseEnvelope(body)
			require.NoError(t, err)
			require.NotEmpty(t, items)

			foundEvent := false
			for _, item := range items {
				if item.Type != "event" {
					continue
				}
				var evt processor.Event
				require.NoError(t, json.Unmarshal(item.Payload, &evt))
				require.NotEmpty(t, evt.EventID)
				require.NotEmpty(t, evt.Platform)
				foundEvent = true
			}
			require.True(t, foundEvent)
		})
	}
}

func TestHandleEnvelopeGzipBody(t *testing.T) {
	store := &fakeStore{
		project: storage.ProjectRef{
			ProjectID: "1",
			PublicKey: "public123",
		},
	}
	handler := New(store, processor.New(store), slog.New(slog.NewTextHandler(io.Discard, nil)), hub.New(), ratelimit.New(1000))

	raw := []byte("{\"event_id\":\"evt-gz\"}\n{\"type\":\"event\"}\n{\"event_id\":\"evt-gz\",\"platform\":\"python\",\"message\":\"gzip test\"}\n")
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	_, err := gz.Write(raw)
	require.NoError(t, err)
	require.NoError(t, gz.Close())

	req := httptest.NewRequest(http.MethodPost, "/api/1/envelope/?sentry_key=public123", &buf)
	req.Header.Set("Content-Encoding", "gzip")
	req.SetPathValue("project_id", "1")
	rec := httptest.NewRecorder()

	handler.HandleEnvelope(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.NotNil(t, store.lastRecord)
	require.Equal(t, "evt-gz", store.lastRecord.EventID)
}

func TestHandleEnvelopeAuthFromEnvelopeHeader(t *testing.T) {
	store := &fakeStore{
		project: storage.ProjectRef{
			ProjectID: "1",
			PublicKey: "public123",
		},
	}
	handler := New(store, processor.New(store), slog.New(slog.NewTextHandler(io.Discard, nil)), hub.New(), ratelimit.New(1000))

	// No sentry_key in query or HTTP headers; DSN is in the envelope header line.
	envelope := []byte("{\"event_id\":\"evt-dsn\",\"dsn\":\"http://public123@localhost/1\"}\n{\"type\":\"event\"}\n{\"event_id\":\"evt-dsn\",\"platform\":\"node\",\"message\":\"dsn from envelope\"}\n")
	req := httptest.NewRequest(http.MethodPost, "/api/1/envelope/", bytes.NewReader(envelope))
	req.SetPathValue("project_id", "1")
	rec := httptest.NewRecorder()

	handler.HandleEnvelope(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.NotNil(t, store.lastRecord)
	require.Equal(t, "evt-dsn", store.lastRecord.EventID)
}

func TestExtractPublicKeyHeaderBeforeQuery(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/1/envelope/?sentry_key=fromquery", nil)
	req.Header.Set("X-Sentry-Auth", "Sentry sentry_key=fromheader, sentry_version=7")

	require.Equal(t, "fromheader", extractPublicKey(req))
}

type fakeStore struct {
	project    storage.ProjectRef
	lastRecord *storage.RecordProcessedEventParams
}

func (f *fakeStore) Close() error                  { return nil }
func (f *fakeStore) Migrate(context.Context) error { return nil }
func (f *fakeStore) Bootstrap(context.Context, storage.BootstrapParams) (storage.BootstrapResult, error) {
	return storage.BootstrapResult{}, nil
}
func (f *fakeStore) GetUserByEmail(context.Context, string) (storage.User, error) {
	return storage.User{}, storage.ErrNotFound
}
func (f *fakeStore) CreateSession(context.Context, storage.CreateSessionParams) (storage.Session, error) {
	return storage.Session{}, nil
}
func (f *fakeStore) GetSessionByTokenHash(context.Context, string) (storage.Session, error) {
	return storage.Session{}, storage.ErrNotFound
}
func (f *fakeStore) DeleteSessionByTokenHash(context.Context, string) error { return nil }
func (f *fakeStore) DeleteExpiredSessions(context.Context, int64) error     { return nil }
func (f *fakeStore) UpdateIssueStatus(context.Context, string, string, int64) error { return nil }
func (f *fakeStore) AuthenticateProjectKey(_ context.Context, projectID, publicKey string) (storage.ProjectRef, error) {
	if projectID == f.project.ProjectID && publicKey == f.project.PublicKey {
		return f.project, nil
	}
	return storage.ProjectRef{}, storage.ErrNotFound
}
func (f *fakeStore) RecordProcessedEvent(_ context.Context, params storage.RecordProcessedEventParams) (storage.RecordProcessedEventResult, error) {
	record := params
	f.lastRecord = &record
	return storage.RecordProcessedEventResult{
		IssueID: "issue-1",
		EventID: params.EventID,
	}, nil
}
func (f *fakeStore) ListIssues(context.Context, storage.ListIssuesFilter, int) ([]storage.IssueSummary, error) {
	return nil, nil
}
func (f *fakeStore) GetIssue(context.Context, string) (storage.IssueDetail, error) {
	return storage.IssueDetail{}, storage.ErrNotFound
}
func (f *fakeStore) ListEventsByIssue(context.Context, string, int) ([]storage.EventRecord, error) {
	return nil, nil
}
func (f *fakeStore) ListDistinctEnvironments(context.Context) ([]string, error) {
	return nil, nil
}
func (f *fakeStore) StoreAttachment(context.Context, storage.AttachmentParams) error { return nil }
func (f *fakeStore) ListAttachmentsByEventID(context.Context, string) ([]storage.AttachmentMeta, error) {
	return nil, nil
}
func (f *fakeStore) GetAttachmentData(context.Context, string) ([]byte, string, error) {
	return nil, "", storage.ErrNotFound
}
func (f *fakeStore) StoreSourceMap(context.Context, storage.SourceMapParams) error { return nil }
func (f *fakeStore) GetSourceMap(context.Context, string, string, string) (string, error) {
	return "", storage.ErrNotFound
}
func (f *fakeStore) GetDashboardStats(context.Context) (storage.DashboardStats, error) {
	return storage.DashboardStats{}, nil
}
func (f *fakeStore) GetEventVolumeByDay(context.Context, int) ([]storage.DayCount, error) {
	return nil, nil
}
func (f *fakeStore) ListReleaseStats(context.Context) ([]storage.ReleaseStat, error) {
	return nil, nil
}
