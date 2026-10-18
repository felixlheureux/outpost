-- +goose Up
CREATE TABLE issue_sources (
    project_id TEXT NOT NULL REFERENCES projects(id),
    node_id TEXT NOT NULL,
    github_id INTEGER NOT NULL,
    number INTEGER NOT NULL CHECK (number > 0),
    url TEXT NOT NULL,
    title TEXT NOT NULL,
    body TEXT NOT NULL,
    state TEXT NOT NULL CHECK (state IN ('open', 'closed')),
    source_updated_at TEXT NOT NULL,
    content_digest TEXT NOT NULL,
    fetched_at TEXT NOT NULL,
    local_revision INTEGER NOT NULL CHECK (local_revision > 0),
    requirements_json TEXT NOT NULL,
    parse_error TEXT,
    PRIMARY KEY (project_id, node_id),
    UNIQUE (project_id, number)
);
CREATE TABLE issue_source_status (
    project_id TEXT PRIMARY KEY REFERENCES projects(id),
    state TEXT NOT NULL CHECK (state IN ('ready', 'needs_attention')),
    last_success_at TEXT,
    last_error TEXT
);
CREATE TABLE issue_operations (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL REFERENCES projects(id),
    request_key TEXT NOT NULL,
    kind TEXT NOT NULL CHECK (kind IN ('create', 'edit')),
    issue_number INTEGER,
    expected_digest TEXT,
    expected_revision INTEGER,
    title TEXT NOT NULL,
    body TEXT NOT NULL,
    marker TEXT NOT NULL,
    state TEXT NOT NULL CHECK (state IN ('pending', 'confirmed', 'rejected', 'conflict', 'unknown')),
    result_number INTEGER,
    reason TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    UNIQUE (project_id, request_key)
);

-- +goose Down
DROP TABLE issue_operations;
DROP TABLE issue_source_status;
DROP TABLE issue_sources;
