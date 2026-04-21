package storage

import (
	"context"
	"errors"
)

var (
	// ErrNotFound indicates that no matching row exists for the requested record.
	ErrNotFound = errors.New("storage: not found")
	// ErrAlreadyInitialized is returned when bootstrap is attempted on a non-empty DB.
	ErrAlreadyInitialized = errors.New("storage: already initialized")
)

// Store exposes the persistence API consumed by the rest of the application.
type Store interface {
	Close() error
	Migrate(ctx context.Context) error
	Bootstrap(ctx context.Context, params BootstrapParams) (BootstrapResult, error)
	GetUserByEmail(ctx context.Context, email string) (User, error)
	CreateSession(ctx context.Context, params CreateSessionParams) (Session, error)
	GetSessionByTokenHash(ctx context.Context, tokenHash string) (Session, error)
	DeleteSessionByTokenHash(ctx context.Context, tokenHash string) error
	DeleteExpiredSessions(ctx context.Context, now int64) error
	AuthenticateProjectKey(ctx context.Context, projectID, publicKey string) (ProjectRef, error)
	RecordProcessedEvent(ctx context.Context, params RecordProcessedEventParams) (RecordProcessedEventResult, error)
	UpdateIssueStatus(ctx context.Context, issueID, status string, updatedAt int64) error
	ListIssues(ctx context.Context, filter ListIssuesFilter, limit int) ([]IssueSummary, error)
	ListDistinctEnvironments(ctx context.Context) ([]string, error)
	StoreAttachment(ctx context.Context, params AttachmentParams) error
	ListAttachmentsByEventID(ctx context.Context, eventID string) ([]AttachmentMeta, error)
	GetAttachmentData(ctx context.Context, attachmentID string) ([]byte, string, error)
	StoreSourceMap(ctx context.Context, params SourceMapParams) error
	GetSourceMap(ctx context.Context, projectID, release, filename string) (string, error)
	GetIssue(ctx context.Context, issueID string) (IssueDetail, error)
	ListEventsByIssue(ctx context.Context, issueID string, limit int) ([]EventRecord, error)
	GetDashboardStats(ctx context.Context) (DashboardStats, error)
	GetEventVolumeByDay(ctx context.Context, days int) ([]DayCount, error)
	ListReleaseStats(ctx context.Context) ([]ReleaseStat, error)
}

// BootstrapParams contains the first-run data created by `bugrail init`.
type BootstrapParams struct {
	AdminEmail   string
	PasswordHash string
	OrgName      string
	OrgSlug      string
	ProjectName  string
	ProjectSlug  string
}

// BootstrapResult describes the created installation bootstrap artifacts.
type BootstrapResult struct {
	OrganizationID string
	ProjectID      string
	PublicKey      string
	SecretKey      string
}

// User represents an application user.
type User struct {
	ID           string
	Email        string
	PasswordHash string
	CreatedAt    int64
}

// CreateSessionParams creates a persisted login session.
type CreateSessionParams struct {
	UserID    string
	TokenHash string
	CreatedAt int64
	ExpiresAt int64
}

// Session represents a persisted authenticated session.
type Session struct {
	ID        string
	UserID    string
	Email     string
	TokenHash string
	CreatedAt int64
	ExpiresAt int64
}

// ProjectRef identifies a validated ingestion target.
type ProjectRef struct {
	ProjectID   string
	ProjectName string
	PublicKey   string
}

// RecordProcessedEventParams contains the normalized event ready for persistence.
type RecordProcessedEventParams struct {
	ProjectID      string
	EventID        string
	GroupingKey    string
	IssueTitle     string
	Platform       string
	Environment    string
	Release        string
	Level          string
	Culprit        string
	ExceptionType  string
	ExceptionValue string
	Fingerprint    string
	Payload        string
	ReceivedAt     int64
}

// RecordProcessedEventResult describes the storage outcome of an ingested event.
type RecordProcessedEventResult struct {
	IssueID   string
	EventID   string
	Duplicate bool
	NewIssue  bool // true when the issue was created by this event (first occurrence)
}

// ListIssuesFilter holds optional filters for the issues list.
type ListIssuesFilter struct {
	Status      string // "open", "resolved", "ignored", or "" for all
	Platform    string // "" for all
	Environment string // "" for all
	Level       string // "fatal", "error", "warning", "info", "debug", or "" for all
	Search      string // partial match on title and culprit; "" for all
	Cursor      int64  // last_seen_at of last seen item for cursor pagination; 0 = first page
}

// IssueSummary is used by the issues list page.
type IssueSummary struct {
	ID          string
	ProjectID   string
	ProjectName string
	Title       string
	Culprit     string
	Platform    string
	Environment string
	Level       string
	Status      string
	FirstSeenAt int64
	LastSeenAt  int64
	EventCount  int64
	LastEventID string
}

// IssueDetail is used by the issue detail page.
type IssueDetail struct {
	IssueSummary
	GroupingKey string
}

// SourceMapParams contains data for storing a source map.
type SourceMapParams struct {
	ID        string
	ProjectID string
	Release   string
	Filename  string
	Content   string
	CreatedAt int64
}

// AttachmentParams contains data for storing a new attachment.
type AttachmentParams struct {
	ID          string
	EventID     string
	ProjectID   string
	Filename    string
	ContentType string
	Data        []byte
	CreatedAt   int64
}

// AttachmentMeta is the list-safe view of an attachment (no blob data).
type AttachmentMeta struct {
	ID          string
	EventID     string
	Filename    string
	ContentType string
	Size        int64
}

// DashboardStats holds pre-aggregated counts and breakdowns for the dashboard page.
type DashboardStats struct {
	OpenIssues      int64
	FatalOpenIssues int64
	Events24h       int64
	ResolvedLast7d  int64
	LevelCounts     []LevelCount
	PlatformCounts  []PlatformCount
}

// LevelCount is one row of the issues-by-level breakdown.
type LevelCount struct {
	Level string
	Count int64
}

// PlatformCount is one row of the issues-by-platform breakdown.
type PlatformCount struct {
	Platform string
	Count    int64
}

// DayCount is one data point for the event-volume-by-day chart.
type DayCount struct {
	Day   string // "2006-01-02"
	Count int64
}

// ReleaseStat is one row of the releases table.
type ReleaseStat struct {
	Release    string
	IssueCount int64
	EventCount int64
	LastSeen   int64
}

// EventRecord is used by the issue detail page.
type EventRecord struct {
	ID             string
	IssueID        string
	ProjectID      string
	EventID        string
	Platform       string
	Environment    string
	Level          string
	Title          string
	Culprit        string
	ExceptionType  string
	ExceptionValue string
	Fingerprint    string
	Payload        string
	Release        string
	Attachments    []AttachmentMeta
	ReceivedAt     int64
}
