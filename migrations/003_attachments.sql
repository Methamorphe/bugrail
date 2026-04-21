-- +goose Up
CREATE TABLE attachments (
    id           TEXT PRIMARY KEY,
    event_id     TEXT NOT NULL,
    project_id   TEXT NOT NULL,
    filename     TEXT NOT NULL DEFAULT '',
    content_type TEXT NOT NULL DEFAULT '',
    size         INTEGER NOT NULL DEFAULT 0,
    data         BLOB NOT NULL,
    created_at   INTEGER NOT NULL,
    FOREIGN KEY (project_id) REFERENCES projects(id)
);
CREATE INDEX attachments_event_id ON attachments(event_id);

-- +goose Down
DROP INDEX IF EXISTS attachments_event_id;
DROP TABLE IF EXISTS attachments;
