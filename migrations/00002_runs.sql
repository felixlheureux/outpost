-- +goose Up
CREATE TABLE runs (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL REFERENCES projects(id),
    request_key TEXT NOT NULL UNIQUE,
    workspace_path TEXT NOT NULL,
    launch_token TEXT NOT NULL UNIQUE,
    unit_name TEXT NOT NULL UNIQUE,
    state TEXT NOT NULL CHECK (state IN ('reserved', 'running', 'interrupted', 'completed', 'stopped')),
    launch_observation TEXT NOT NULL CHECK (launch_observation IN ('pending', 'acknowledged', 'unknown')),
    worker_pid INTEGER,
    worker_start_ticks TEXT,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE UNIQUE INDEX one_active_run ON runs((1))
    WHERE state IN ('reserved', 'running', 'interrupted');

-- +goose Down
DROP INDEX one_active_run;
DROP TABLE runs;
