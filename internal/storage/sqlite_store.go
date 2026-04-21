package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/Methamorphe/bugrail/internal/config"
	"github.com/Methamorphe/bugrail/internal/storage/sqlitegen"
)

type sqliteStore struct {
	db      *sql.DB
	queries *sqlitegen.Queries
}

func newSQLiteStore(db *sql.DB) Store {
	return &sqliteStore{
		db:      db,
		queries: sqlitegen.New(db),
	}
}

func (s *sqliteStore) Close() error {
	return s.db.Close()
}

func (s *sqliteStore) Migrate(ctx context.Context) error {
	return runMigrations(ctx, s.db, config.DriverSQLite)
}

func (s *sqliteStore) Bootstrap(ctx context.Context, params BootstrapParams) (BootstrapResult, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return BootstrapResult{}, fmt.Errorf("begin bootstrap tx: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	q := s.queries.WithTx(tx)
	userCount, err := q.CountUsers(ctx)
	if err != nil {
		return BootstrapResult{}, fmt.Errorf("count users: %w", err)
	}
	projectCount, err := q.CountProjects(ctx)
	if err != nil {
		return BootstrapResult{}, fmt.Errorf("count projects: %w", err)
	}
	if userCount > 0 || projectCount > 0 {
		return BootstrapResult{}, ErrAlreadyInitialized
	}

	now := unixNow()
	orgID, err := randomHex(12)
	if err != nil {
		return BootstrapResult{}, err
	}
	userID, err := randomHex(12)
	if err != nil {
		return BootstrapResult{}, err
	}
	projectKeyID, err := randomHex(12)
	if err != nil {
		return BootstrapResult{}, err
	}
	publicKey, err := randomHex(16)
	if err != nil {
		return BootstrapResult{}, err
	}
	secretKey, err := randomHex(16)
	if err != nil {
		return BootstrapResult{}, err
	}
	projectID := projectNumericID(projectCount)

	if err := q.CreateUser(ctx, sqlitegen.CreateUserParams{
		ID:           userID,
		Email:        params.AdminEmail,
		PasswordHash: params.PasswordHash,
		CreatedAt:    now,
	}); err != nil {
		return BootstrapResult{}, fmt.Errorf("create admin user: %w", err)
	}
	if err := q.CreateOrganization(ctx, sqlitegen.CreateOrganizationParams{
		ID:        orgID,
		Slug:      params.OrgSlug,
		Name:      params.OrgName,
		CreatedAt: now,
	}); err != nil {
		return BootstrapResult{}, fmt.Errorf("create organization: %w", err)
	}
	if err := q.CreateProject(ctx, sqlitegen.CreateProjectParams{
		ID:             projectID,
		OrganizationID: orgID,
		Slug:           params.ProjectSlug,
		Name:           params.ProjectName,
		CreatedAt:      now,
	}); err != nil {
		return BootstrapResult{}, fmt.Errorf("create project: %w", err)
	}
	if err := q.CreateProjectKey(ctx, sqlitegen.CreateProjectKeyParams{
		ID:        projectKeyID,
		ProjectID: projectID,
		PublicKey: publicKey,
		SecretKey: secretKey,
		CreatedAt: now,
	}); err != nil {
		return BootstrapResult{}, fmt.Errorf("create project key: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return BootstrapResult{}, fmt.Errorf("commit bootstrap tx: %w", err)
	}

	return BootstrapResult{
		OrganizationID: orgID,
		ProjectID:      projectID,
		PublicKey:      publicKey,
		SecretKey:      secretKey,
	}, nil
}

func (s *sqliteStore) GetUserByEmail(ctx context.Context, email string) (User, error) {
	row, err := s.queries.GetUserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return User{}, ErrNotFound
		}
		return User{}, fmt.Errorf("get user by email: %w", err)
	}
	return User{
		ID:           row.ID,
		Email:        row.Email,
		PasswordHash: row.PasswordHash,
		CreatedAt:    row.CreatedAt,
	}, nil
}

func (s *sqliteStore) CreateSession(ctx context.Context, params CreateSessionParams) (Session, error) {
	sessionID, err := randomHex(12)
	if err != nil {
		return Session{}, err
	}
	if err := s.queries.CreateSession(ctx, sqlitegen.CreateSessionParams{
		ID:        sessionID,
		UserID:    params.UserID,
		TokenHash: params.TokenHash,
		CreatedAt: params.CreatedAt,
		ExpiresAt: params.ExpiresAt,
	}); err != nil {
		return Session{}, fmt.Errorf("create session: %w", err)
	}
	return s.GetSessionByTokenHash(ctx, params.TokenHash)
}

func (s *sqliteStore) GetSessionByTokenHash(ctx context.Context, tokenHash string) (Session, error) {
	row, err := s.queries.GetSessionByTokenHash(ctx, tokenHash)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Session{}, ErrNotFound
		}
		return Session{}, fmt.Errorf("get session by token hash: %w", err)
	}
	return Session{
		ID:        row.ID,
		UserID:    row.UserID,
		Email:     row.Email,
		TokenHash: row.TokenHash,
		CreatedAt: row.CreatedAt,
		ExpiresAt: row.ExpiresAt,
	}, nil
}

func (s *sqliteStore) DeleteSessionByTokenHash(ctx context.Context, tokenHash string) error {
	if err := s.queries.DeleteSessionByTokenHash(ctx, tokenHash); err != nil {
		return fmt.Errorf("delete session by token hash: %w", err)
	}
	return nil
}

func (s *sqliteStore) DeleteExpiredSessions(ctx context.Context, now int64) error {
	if err := s.queries.DeleteExpiredSessions(ctx, now); err != nil {
		return fmt.Errorf("delete expired sessions: %w", err)
	}
	return nil
}

func (s *sqliteStore) AuthenticateProjectKey(ctx context.Context, projectID, publicKey string) (ProjectRef, error) {
	row, err := s.queries.GetProjectKeyByPublicKey(ctx, sqlitegen.GetProjectKeyByPublicKeyParams{
		ProjectID: projectID,
		PublicKey: publicKey,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ProjectRef{}, ErrNotFound
		}
		return ProjectRef{}, fmt.Errorf("get project key: %w", err)
	}
	return ProjectRef{
		ProjectID:   row.ProjectID,
		ProjectName: row.ProjectName,
		PublicKey:   row.PublicKey,
	}, nil
}

func (s *sqliteStore) UpdateIssueStatus(ctx context.Context, issueID, status string, updatedAt int64) error {
	if err := s.queries.UpdateIssueStatus(ctx, sqlitegen.UpdateIssueStatusParams{
		ID:        issueID,
		Status:    status,
		UpdatedAt: updatedAt,
	}); err != nil {
		return fmt.Errorf("update issue status: %w", err)
	}
	return nil
}

func (s *sqliteStore) RecordProcessedEvent(ctx context.Context, params RecordProcessedEventParams) (RecordProcessedEventResult, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return RecordProcessedEventResult{}, fmt.Errorf("begin record event tx: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	q := s.queries.WithTx(tx)
	existingEvent, err := q.GetEventByProjectAndEventID(ctx, sqlitegen.GetEventByProjectAndEventIDParams{
		ProjectID: params.ProjectID,
		EventID:   params.EventID,
	})
	if err == nil {
		if err := tx.Commit(); err != nil {
			return RecordProcessedEventResult{}, fmt.Errorf("commit duplicate event tx: %w", err)
		}
		return RecordProcessedEventResult{
			IssueID:   existingEvent.IssueID,
			EventID:   existingEvent.EventID,
			Duplicate: true,
		}, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return RecordProcessedEventResult{}, fmt.Errorf("lookup existing event: %w", err)
	}

	issue, err := q.GetIssueByGroupingKey(ctx, sqlitegen.GetIssueByGroupingKeyParams{
		ProjectID:   params.ProjectID,
		GroupingKey: params.GroupingKey,
	})
	now := params.ReceivedAt
	newIssue := false
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			return RecordProcessedEventResult{}, fmt.Errorf("lookup issue by grouping key: %w", err)
		}
		newIssue = true

		issueID, idErr := randomHex(12)
		if idErr != nil {
			return RecordProcessedEventResult{}, idErr
		}
		if err := q.CreateIssue(ctx, sqlitegen.CreateIssueParams{
			ID:          issueID,
			ProjectID:   params.ProjectID,
			GroupingKey: params.GroupingKey,
			Title:       params.IssueTitle,
			Culprit:     params.Culprit,
			Platform:    params.Platform,
			Environment: params.Environment,
			Level:       params.Level,
			Status:      "open",
			FirstSeenAt: now,
			LastSeenAt:  now,
			EventCount:  1,
			LastEventID: params.EventID,
			CreatedAt:   now,
			UpdatedAt:   now,
		}); err != nil {
			return RecordProcessedEventResult{}, fmt.Errorf("create issue: %w", err)
		}
		issue = sqlitegen.Issue{ID: issueID}
	} else {
		if err := q.UpdateIssueAfterEvent(ctx, sqlitegen.UpdateIssueAfterEventParams{
			Title:       params.IssueTitle,
			Culprit:     params.Culprit,
			Platform:    params.Platform,
			Environment: params.Environment,
			Level:       params.Level,
			LastSeenAt:  now,
			EventCount:  issue.EventCount + 1,
			LastEventID: params.EventID,
			UpdatedAt:   now,
			ID:          issue.ID,
		}); err != nil {
			return RecordProcessedEventResult{}, fmt.Errorf("update issue after event: %w", err)
		}
	}

	eventRowID, err := randomHex(12)
	if err != nil {
		return RecordProcessedEventResult{}, err
	}
	if err := q.CreateEvent(ctx, sqlitegen.CreateEventParams{
		ID:             eventRowID,
		IssueID:        issue.ID,
		ProjectID:      params.ProjectID,
		EventID:        params.EventID,
		Platform:       params.Platform,
		Environment:    params.Environment,
		Release:        params.Release,
		Level:          params.Level,
		Title:          params.IssueTitle,
		Culprit:        params.Culprit,
		ExceptionType:  params.ExceptionType,
		ExceptionValue: params.ExceptionValue,
		Fingerprint:    params.Fingerprint,
		Payload:        params.Payload,
		ReceivedAt:     now,
	}); err != nil {
		if isDuplicateConstraint(err) {
			if err := tx.Commit(); err != nil {
				return RecordProcessedEventResult{}, fmt.Errorf("commit duplicate create event tx: %w", err)
			}
			return RecordProcessedEventResult{
				IssueID:   issue.ID,
				EventID:   params.EventID,
				Duplicate: true,
			}, nil
		}
		return RecordProcessedEventResult{}, fmt.Errorf("create event: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return RecordProcessedEventResult{}, fmt.Errorf("commit record event tx: %w", err)
	}
	return RecordProcessedEventResult{
		IssueID:  issue.ID,
		EventID:  params.EventID,
		NewIssue: newIssue,
	}, nil
}

func (s *sqliteStore) ListIssues(ctx context.Context, filter ListIssuesFilter, limit int) ([]IssueSummary, error) {
	// Dynamic WHERE: sqlc cannot express optional filter columns, so the query is built here.
	q := `SELECT i.id, i.project_id, p.name AS project_name, i.title, i.culprit,
	       i.platform, i.environment, i.level, i.status, i.first_seen_at, i.last_seen_at,
	       i.event_count, i.last_event_id
	      FROM issues AS i JOIN projects AS p ON p.id = i.project_id WHERE 1=1`
	var args []any
	n := 1
	if filter.Status != "" {
		q += fmt.Sprintf(" AND i.status = ?%d", n)
		args = append(args, filter.Status)
		n++
	}
	if filter.Platform != "" {
		q += fmt.Sprintf(" AND i.platform = ?%d", n)
		args = append(args, filter.Platform)
		n++
	}
	if filter.Environment != "" {
		q += fmt.Sprintf(" AND i.environment = ?%d", n)
		args = append(args, filter.Environment)
		n++
	}
	if filter.Level != "" {
		q += fmt.Sprintf(" AND i.level = ?%d", n)
		args = append(args, filter.Level)
		n++
	}
	if filter.Search != "" {
		q += fmt.Sprintf(" AND (i.title LIKE '%%'||?%d||'%%' OR i.culprit LIKE '%%'||?%d||'%%')", n, n)
		args = append(args, filter.Search)
		n++
	}
	if filter.Cursor > 0 {
		q += fmt.Sprintf(" AND i.last_seen_at < ?%d", n)
		args = append(args, filter.Cursor)
		n++
	}
	q += fmt.Sprintf(" ORDER BY i.last_seen_at DESC LIMIT ?%d", n)
	args = append(args, normalizeLimit(limit, 50))

	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("list issues: %w", err)
	}
	defer rows.Close()
	var items []IssueSummary
	for rows.Next() {
		var i IssueSummary
		if err := rows.Scan(&i.ID, &i.ProjectID, &i.ProjectName, &i.Title, &i.Culprit,
			&i.Platform, &i.Environment, &i.Level, &i.Status, &i.FirstSeenAt, &i.LastSeenAt,
			&i.EventCount, &i.LastEventID); err != nil {
			return nil, fmt.Errorf("scan issue row: %w", err)
		}
		items = append(items, i)
	}
	return items, rows.Err()
}

func (s *sqliteStore) StoreAttachment(ctx context.Context, p AttachmentParams) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO attachments (id, event_id, project_id, filename, content_type, size, data, created_at)
		 VALUES (?1, ?2, ?3, ?4, ?5, ?6, ?7, ?8)`,
		p.ID, p.EventID, p.ProjectID, p.Filename, p.ContentType, len(p.Data), p.Data, p.CreatedAt,
	)
	return err
}

func (s *sqliteStore) ListAttachmentsByEventID(ctx context.Context, eventID string) ([]AttachmentMeta, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, event_id, filename, content_type, size FROM attachments WHERE event_id = ?1 ORDER BY rowid`,
		eventID,
	)
	if err != nil {
		return nil, fmt.Errorf("list attachments: %w", err)
	}
	defer rows.Close()
	var out []AttachmentMeta
	for rows.Next() {
		var a AttachmentMeta
		if err := rows.Scan(&a.ID, &a.EventID, &a.Filename, &a.ContentType, &a.Size); err != nil {
			return nil, fmt.Errorf("scan attachment: %w", err)
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (s *sqliteStore) GetAttachmentData(ctx context.Context, attachmentID string) ([]byte, string, error) {
	var data []byte
	var ct string
	err := s.db.QueryRowContext(ctx,
		`SELECT data, content_type FROM attachments WHERE id = ?1`, attachmentID,
	).Scan(&data, &ct)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, "", ErrNotFound
		}
		return nil, "", fmt.Errorf("get attachment: %w", err)
	}
	return data, ct, nil
}

func (s *sqliteStore) ListDistinctEnvironments(ctx context.Context) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT DISTINCT environment FROM issues WHERE environment != '' ORDER BY environment`)
	if err != nil {
		return nil, fmt.Errorf("list environments: %w", err)
	}
	defer rows.Close()
	var envs []string
	for rows.Next() {
		var e string
		if err := rows.Scan(&e); err != nil {
			return nil, fmt.Errorf("scan environment: %w", err)
		}
		envs = append(envs, e)
	}
	return envs, rows.Err()
}

func (s *sqliteStore) StoreSourceMap(ctx context.Context, p SourceMapParams) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO source_maps (id, project_id, release, filename, content, created_at)
		 VALUES (?1, ?2, ?3, ?4, ?5, ?6)
		 ON CONFLICT(project_id, release, filename) DO UPDATE SET content = excluded.content`,
		p.ID, p.ProjectID, p.Release, p.Filename, p.Content, p.CreatedAt,
	)
	return err
}

func (s *sqliteStore) GetSourceMap(ctx context.Context, projectID, release, filename string) (string, error) {
	var content string
	err := s.db.QueryRowContext(ctx,
		`SELECT content FROM source_maps WHERE project_id = ?1 AND release = ?2 AND filename = ?3`,
		projectID, release, filename,
	).Scan(&content)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", ErrNotFound
		}
		return "", fmt.Errorf("get source map: %w", err)
	}
	return content, nil
}

func (s *sqliteStore) GetIssue(ctx context.Context, issueID string) (IssueDetail, error) {
	row, err := s.queries.GetIssueByID(ctx, issueID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return IssueDetail{}, ErrNotFound
		}
		return IssueDetail{}, fmt.Errorf("get issue: %w", err)
	}
	return IssueDetail{
		IssueSummary: IssueSummary{
			ID:          row.ID,
			ProjectID:   row.ProjectID,
			ProjectName: row.ProjectName,
			Title:       row.Title,
			Culprit:     row.Culprit,
			Platform:    row.Platform,
			Environment: row.Environment,
			Status:      row.Status,
			FirstSeenAt: row.FirstSeenAt,
			LastSeenAt:  row.LastSeenAt,
			EventCount:  row.EventCount,
			LastEventID: row.LastEventID,
		},
		GroupingKey: row.GroupingKey,
	}, nil
}

func (s *sqliteStore) ListEventsByIssue(ctx context.Context, issueID string, limit int) ([]EventRecord, error) {
	rows, err := s.queries.ListEventsByIssue(ctx, sqlitegen.ListEventsByIssueParams{
		IssueID:    issueID,
		LimitCount: normalizeLimit(limit, 50),
	})
	if err != nil {
		return nil, fmt.Errorf("list events by issue: %w", err)
	}
	items := make([]EventRecord, 0, len(rows))
	for _, row := range rows {
		items = append(items, EventRecord{
			ID:             row.ID,
			IssueID:        row.IssueID,
			ProjectID:      row.ProjectID,
			EventID:        row.EventID,
			Platform:       row.Platform,
			Environment:    row.Environment,
			Release:        row.Release,
			Level:          row.Level,
			Title:          row.Title,
			Culprit:        row.Culprit,
			ExceptionType:  row.ExceptionType,
			ExceptionValue: row.ExceptionValue,
			Fingerprint:    row.Fingerprint,
			Payload:        row.Payload,
			ReceivedAt:     row.ReceivedAt,
		})
	}
	return items, nil
}

func (s *sqliteStore) GetDashboardStats(ctx context.Context) (DashboardStats, error) {
	var st DashboardStats
	now := unixNow()

	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM issues WHERE status='open'`).Scan(&st.OpenIssues); err != nil {
		return st, fmt.Errorf("open issues count: %w", err)
	}
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM issues WHERE status='open' AND level='fatal'`).Scan(&st.FatalOpenIssues); err != nil {
		return st, fmt.Errorf("fatal open issues count: %w", err)
	}
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM events WHERE received_at > ?1`, now-86400).Scan(&st.Events24h); err != nil {
		return st, fmt.Errorf("events 24h count: %w", err)
	}
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM issues WHERE status='resolved' AND updated_at > ?1`, now-7*86400).Scan(&st.ResolvedLast7d); err != nil {
		return st, fmt.Errorf("resolved 7d count: %w", err)
	}

	rows, err := s.db.QueryContext(ctx, `SELECT level, COUNT(*) FROM issues WHERE status='open' GROUP BY level ORDER BY COUNT(*) DESC`)
	if err != nil {
		return st, fmt.Errorf("level counts: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var lc LevelCount
		if err := rows.Scan(&lc.Level, &lc.Count); err != nil {
			return st, err
		}
		st.LevelCounts = append(st.LevelCounts, lc)
	}
	if err := rows.Err(); err != nil {
		return st, err
	}

	rows2, err := s.db.QueryContext(ctx, `SELECT platform, COUNT(*) FROM issues WHERE status='open' GROUP BY platform ORDER BY COUNT(*) DESC LIMIT 5`)
	if err != nil {
		return st, fmt.Errorf("platform counts: %w", err)
	}
	defer rows2.Close()
	for rows2.Next() {
		var pc PlatformCount
		if err := rows2.Scan(&pc.Platform, &pc.Count); err != nil {
			return st, err
		}
		st.PlatformCounts = append(st.PlatformCounts, pc)
	}
	return st, rows2.Err()
}

func (s *sqliteStore) GetEventVolumeByDay(ctx context.Context, days int) ([]DayCount, error) {
	since := unixNow() - int64(days)*86400
	rows, err := s.db.QueryContext(ctx,
		`SELECT date(datetime(received_at,'unixepoch')) AS day, COUNT(*)
		 FROM events WHERE received_at > ?1
		 GROUP BY day ORDER BY day`,
		since,
	)
	if err != nil {
		return nil, fmt.Errorf("event volume by day: %w", err)
	}
	defer rows.Close()
	var out []DayCount
	for rows.Next() {
		var dc DayCount
		if err := rows.Scan(&dc.Day, &dc.Count); err != nil {
			return nil, err
		}
		out = append(out, dc)
	}
	return out, rows.Err()
}

func (s *sqliteStore) ListReleaseStats(ctx context.Context) ([]ReleaseStat, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT release, COUNT(DISTINCT issue_id), COUNT(*), MAX(received_at)
		 FROM events WHERE release != ''
		 GROUP BY release ORDER BY MAX(received_at) DESC LIMIT 50`,
	)
	if err != nil {
		return nil, fmt.Errorf("list release stats: %w", err)
	}
	defer rows.Close()
	var out []ReleaseStat
	for rows.Next() {
		var rs ReleaseStat
		if err := rows.Scan(&rs.Release, &rs.IssueCount, &rs.EventCount, &rs.LastSeen); err != nil {
			return nil, err
		}
		out = append(out, rs)
	}
	return out, rows.Err()
}
