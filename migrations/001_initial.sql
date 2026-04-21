-- +goose Up
CREATE TABLE users (
    id TEXT PRIMARY KEY,
    email TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    created_at BIGINT NOT NULL
);

CREATE TABLE sessions (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    token_hash TEXT NOT NULL UNIQUE,
    created_at BIGINT NOT NULL,
    expires_at BIGINT NOT NULL,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE TABLE organizations (
    id TEXT PRIMARY KEY,
    slug TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    created_at BIGINT NOT NULL
);

CREATE TABLE projects (
    id TEXT PRIMARY KEY,
    organization_id TEXT NOT NULL,
    slug TEXT NOT NULL,
    name TEXT NOT NULL,
    created_at BIGINT NOT NULL,
    FOREIGN KEY (organization_id) REFERENCES organizations(id) ON DELETE CASCADE,
    UNIQUE (organization_id, slug)
);

CREATE TABLE project_keys (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL,
    public_key TEXT NOT NULL UNIQUE,
    secret_key TEXT NOT NULL UNIQUE,
    created_at BIGINT NOT NULL,
    FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE
);

CREATE TABLE issues (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL,
    grouping_key TEXT NOT NULL,
    title TEXT NOT NULL,
    culprit TEXT NOT NULL,
    platform TEXT NOT NULL,
    environment TEXT NOT NULL,
    status TEXT NOT NULL,
    first_seen_at BIGINT NOT NULL,
    last_seen_at BIGINT NOT NULL,
    event_count BIGINT NOT NULL,
    last_event_id TEXT NOT NULL,
    created_at BIGINT NOT NULL,
    updated_at BIGINT NOT NULL,
    FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE,
    UNIQUE (project_id, grouping_key)
);

CREATE TABLE events (
    id TEXT PRIMARY KEY,
    issue_id TEXT NOT NULL,
    project_id TEXT NOT NULL,
    event_id TEXT NOT NULL,
    platform TEXT NOT NULL,
    environment TEXT NOT NULL,
    level TEXT NOT NULL,
    title TEXT NOT NULL,
    culprit TEXT NOT NULL,
    exception_type TEXT NOT NULL,
    exception_value TEXT NOT NULL,
    fingerprint TEXT NOT NULL,
    payload TEXT NOT NULL,
    received_at BIGINT NOT NULL,
    FOREIGN KEY (issue_id) REFERENCES issues(id) ON DELETE CASCADE,
    FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE,
    UNIQUE (project_id, event_id)
);

CREATE INDEX idx_sessions_expires_at ON sessions (expires_at);
CREATE INDEX idx_projects_org_created_at ON projects (organization_id, created_at DESC);
CREATE INDEX idx_project_keys_project_id ON project_keys (project_id);
CREATE INDEX idx_issues_project_last_seen ON issues (project_id, last_seen_at DESC);
CREATE INDEX idx_events_issue_received ON events (issue_id, received_at DESC);
CREATE INDEX idx_events_project_received ON events (project_id, received_at DESC);

-- +goose Down
DROP TABLE IF EXISTS events;
DROP TABLE IF EXISTS issues;
DROP TABLE IF EXISTS project_keys;
DROP TABLE IF EXISTS projects;
DROP TABLE IF EXISTS organizations;
DROP TABLE IF EXISTS sessions;
DROP TABLE IF EXISTS users;
