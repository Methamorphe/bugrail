-- +goose Up
ALTER TABLE events ADD COLUMN release TEXT NOT NULL DEFAULT '';

CREATE TABLE source_maps (
    id         TEXT PRIMARY KEY,
    project_id TEXT NOT NULL,
    release    TEXT NOT NULL,
    filename   TEXT NOT NULL,
    content    TEXT NOT NULL,
    created_at INTEGER NOT NULL,
    FOREIGN KEY (project_id) REFERENCES projects(id),
    UNIQUE(project_id, release, filename)
);
CREATE INDEX source_maps_lookup ON source_maps(project_id, release, filename);

-- +goose Down
DROP INDEX IF EXISTS source_maps_lookup;
DROP TABLE IF EXISTS source_maps;
-- SQLite does not support DROP COLUMN before 3.35; leave the release column.
