-- +goose Up
CREATE TABLE provider_attempts (
    assignment_id TEXT PRIMARY KEY REFERENCES assignments(id),
    attempt_id TEXT NOT NULL UNIQUE REFERENCES assignment_attempts(id),
    state TEXT NOT NULL CHECK (state IN ('pending','thread_started','turn_started','completed','failed','interrupted','unknown')),
    version TEXT NOT NULL,
    model TEXT NOT NULL,
    reasoning TEXT NOT NULL,
    sandbox TEXT NOT NULL,
    approval_policy TEXT NOT NULL,
    prompt TEXT NOT NULL,
    thread_id TEXT NOT NULL DEFAULT '',
    turn_id TEXT NOT NULL DEFAULT '',
    handoff_json TEXT NOT NULL DEFAULT '',
    failure_code TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

-- +goose Down
DROP TABLE provider_attempts;
