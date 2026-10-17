-- +goose Up
CREATE TABLE projects (
    id TEXT PRIMARY KEY,
    github_owner TEXT NOT NULL,
    github_name TEXT NOT NULL,
    repository_url TEXT NOT NULL,
    setup_state TEXT NOT NULL CHECK (setup_state IN ('pending', 'ready', 'failed')),
    created_at TEXT NOT NULL,
    UNIQUE (github_owner COLLATE NOCASE, github_name COLLATE NOCASE)
);

-- +goose Down
DROP TABLE projects;
