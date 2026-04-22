-- name: CountUsers :one
SELECT COUNT(*) FROM users;

-- name: CountProjects :one
SELECT COUNT(*) FROM projects;

-- name: CreateUser :exec
INSERT INTO users (
    id,
    email,
    password_hash,
    created_at
) VALUES (
    sqlc.arg(id),
    sqlc.arg(email),
    sqlc.arg(password_hash),
    sqlc.arg(created_at)
);

-- name: GetUserByEmail :one
SELECT id, email, password_hash, created_at
FROM users
WHERE email = sqlc.arg(email)
LIMIT 1;

-- name: CreateSession :exec
INSERT INTO sessions (
    id,
    user_id,
    token_hash,
    created_at,
    expires_at
) VALUES (
    sqlc.arg(id),
    sqlc.arg(user_id),
    sqlc.arg(token_hash),
    sqlc.arg(created_at),
    sqlc.arg(expires_at)
);

-- name: GetSessionByTokenHash :one
SELECT
    s.id,
    s.user_id,
    s.token_hash,
    s.created_at,
    s.expires_at,
    u.email
FROM sessions AS s
JOIN users AS u ON u.id = s.user_id
WHERE s.token_hash = sqlc.arg(token_hash)
LIMIT 1;

-- name: DeleteSessionByTokenHash :exec
DELETE FROM sessions
WHERE token_hash = sqlc.arg(token_hash);

-- name: DeleteExpiredSessions :exec
DELETE FROM sessions
WHERE expires_at <= sqlc.arg(expires_at);

-- name: CreateOrganization :exec
INSERT INTO organizations (
    id,
    slug,
    name,
    created_at
) VALUES (
    sqlc.arg(id),
    sqlc.arg(slug),
    sqlc.arg(name),
    sqlc.arg(created_at)
);

-- name: CreateProject :exec
INSERT INTO projects (
    id,
    organization_id,
    slug,
    name,
    created_at
) VALUES (
    sqlc.arg(id),
    sqlc.arg(organization_id),
    sqlc.arg(slug),
    sqlc.arg(name),
    sqlc.arg(created_at)
);

-- name: GetProjectByID :one
SELECT id, organization_id, slug, name, created_at
FROM projects
WHERE id = sqlc.arg(id)
LIMIT 1;

-- name: CreateProjectKey :exec
INSERT INTO project_keys (
    id,
    project_id,
    public_key,
    secret_key,
    created_at
) VALUES (
    sqlc.arg(id),
    sqlc.arg(project_id),
    sqlc.arg(public_key),
    sqlc.arg(secret_key),
    sqlc.arg(created_at)
);

-- name: GetProjectKeyByPublicKey :one
SELECT
    pk.id,
    pk.project_id,
    pk.public_key,
    pk.secret_key,
    pk.created_at,
    p.name AS project_name
FROM project_keys AS pk
JOIN projects AS p ON p.id = pk.project_id
WHERE pk.project_id = sqlc.arg(project_id)
  AND pk.public_key = sqlc.arg(public_key)
LIMIT 1;

-- name: GetIssueByGroupingKey :one
SELECT
    id,
    project_id,
    grouping_key,
    title,
    culprit,
    platform,
    environment,
    status,
    first_seen_at,
    last_seen_at,
    event_count,
    last_event_id,
    created_at,
    updated_at
FROM issues
WHERE project_id = sqlc.arg(project_id)
  AND grouping_key = sqlc.arg(grouping_key)
LIMIT 1;

-- name: CreateIssue :exec
INSERT INTO issues (
    id,
    project_id,
    grouping_key,
    title,
    culprit,
    platform,
    environment,
    level,
    status,
    first_seen_at,
    last_seen_at,
    event_count,
    last_event_id,
    created_at,
    updated_at
) VALUES (
    sqlc.arg(id),
    sqlc.arg(project_id),
    sqlc.arg(grouping_key),
    sqlc.arg(title),
    sqlc.arg(culprit),
    sqlc.arg(platform),
    sqlc.arg(environment),
    sqlc.arg(level),
    sqlc.arg(status),
    sqlc.arg(first_seen_at),
    sqlc.arg(last_seen_at),
    sqlc.arg(event_count),
    sqlc.arg(last_event_id),
    sqlc.arg(created_at),
    sqlc.arg(updated_at)
);

-- name: UpdateIssueAfterEvent :exec
UPDATE issues
SET
    title = sqlc.arg(title),
    culprit = sqlc.arg(culprit),
    platform = sqlc.arg(platform),
    environment = sqlc.arg(environment),
    last_seen_at = sqlc.arg(last_seen_at),
    event_count = sqlc.arg(event_count),
    last_event_id = sqlc.arg(last_event_id),
    updated_at = sqlc.arg(updated_at),
    status = CASE WHEN status = 'resolved' THEN 'open' ELSE status END
WHERE id = sqlc.arg(id);

-- name: UpdateIssueStatus :exec
UPDATE issues
SET status = sqlc.arg(status), updated_at = sqlc.arg(updated_at)
WHERE id = sqlc.arg(id);

-- name: CreateEvent :exec
INSERT INTO events (
    id,
    issue_id,
    project_id,
    event_id,
    platform,
    environment,
    level,
    title,
    culprit,
    exception_type,
    exception_value,
    fingerprint,
    payload,
    received_at
) VALUES (
    sqlc.arg(id),
    sqlc.arg(issue_id),
    sqlc.arg(project_id),
    sqlc.arg(event_id),
    sqlc.arg(platform),
    sqlc.arg(environment),
    sqlc.arg(level),
    sqlc.arg(title),
    sqlc.arg(culprit),
    sqlc.arg(exception_type),
    sqlc.arg(exception_value),
    sqlc.arg(fingerprint),
    sqlc.arg(payload),
    sqlc.arg(received_at)
);

-- name: GetEventByProjectAndEventID :one
SELECT
    id,
    issue_id,
    project_id,
    event_id,
    platform,
    environment,
    level,
    title,
    culprit,
    exception_type,
    exception_value,
    fingerprint,
    payload,
    received_at
FROM events
WHERE project_id = sqlc.arg(project_id)
  AND event_id = sqlc.arg(event_id)
LIMIT 1;

-- name: ListIssues :many
SELECT
    i.id,
    i.project_id,
    p.name AS project_name,
    i.title,
    i.culprit,
    i.platform,
    i.environment,
    i.status,
    i.first_seen_at,
    i.last_seen_at,
    i.event_count,
    i.last_event_id
FROM issues AS i
JOIN projects AS p ON p.id = i.project_id
ORDER BY i.last_seen_at DESC
LIMIT sqlc.arg(limit_count);

-- name: GetIssueByID :one
SELECT
    i.id,
    i.project_id,
    p.name AS project_name,
    i.grouping_key,
    i.title,
    i.culprit,
    i.platform,
    i.environment,
    i.status,
    i.first_seen_at,
    i.last_seen_at,
    i.event_count,
    i.last_event_id
FROM issues AS i
JOIN projects AS p ON p.id = i.project_id
WHERE i.id = sqlc.arg(id)
LIMIT 1;

-- name: ListEventsByIssue :many
SELECT
    id,
    issue_id,
    project_id,
    event_id,
    platform,
    environment,
    level,
    title,
    culprit,
    exception_type,
    exception_value,
    fingerprint,
    payload,
    received_at
FROM events
WHERE issue_id = sqlc.arg(issue_id)
ORDER BY received_at DESC
LIMIT sqlc.arg(limit_count);
