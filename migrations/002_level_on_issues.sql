-- +goose Up
ALTER TABLE issues ADD COLUMN level TEXT NOT NULL DEFAULT 'error';

-- +goose Down
-- SQLite does not support DROP COLUMN before 3.35; leave it.
