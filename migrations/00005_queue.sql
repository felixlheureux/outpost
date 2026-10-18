-- +goose Up
CREATE TABLE queue_sources (
    project_id TEXT PRIMARY KEY REFERENCES projects(id),
    blob_sha TEXT NOT NULL DEFAULT '',
    issues_json TEXT NOT NULL DEFAULT '[]',
    state TEXT NOT NULL CHECK (state IN ('ready', 'needs_attention')),
    fetched_at TEXT NOT NULL DEFAULT '',
    last_error TEXT NOT NULL DEFAULT ''
);
CREATE TABLE queue_operations (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL REFERENCES projects(id),
    request_key TEXT NOT NULL,
    kind TEXT NOT NULL CHECK (kind IN ('replace', 'append', 'remove', 'move')),
    expected_sha TEXT NOT NULL,
    intended_json TEXT NOT NULL,
    result_json TEXT NOT NULL DEFAULT '[]',
    observed_json TEXT NOT NULL DEFAULT '[]',
    state TEXT NOT NULL CHECK (state IN ('pending', 'confirmed', 'rejected', 'conflict', 'unknown')),
    reason TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    UNIQUE (project_id, request_key)
);

-- +goose Down
DROP TABLE queue_operations;
DROP TABLE queue_sources;
