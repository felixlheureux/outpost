-- +goose Up
ALTER TABLE runs ADD COLUMN reservation_state TEXT NOT NULL DEFAULT 'reserved' CHECK (reservation_state IN ('reserved', 'released'));
UPDATE runs SET reservation_state='released' WHERE state IN ('completed', 'stopped');
DROP INDEX one_active_run;
CREATE UNIQUE INDEX one_active_run ON runs((1)) WHERE reservation_state='reserved';
CREATE TABLE run_schedules (
    run_id TEXT PRIMARY KEY REFERENCES runs(id),
    kind TEXT NOT NULL CHECK (kind IN ('queue', 'task', 'instruction')),
    mode TEXT NOT NULL CHECK (mode IN ('follow', 'fixed', 'single')),
    scope_json TEXT NOT NULL,
    policy_json TEXT NOT NULL,
    instructions TEXT NOT NULL,
    task_limit INTEGER NOT NULL CHECK (task_limit BETWEEN 1 AND 100),
    deadline_at TEXT,
    successful_count INTEGER NOT NULL DEFAULT 0,
    revision INTEGER NOT NULL DEFAULT 1,
    stop_requested INTEGER NOT NULL DEFAULT 0,
    outcome TEXT NOT NULL DEFAULT 'active' CHECK (outcome IN ('active', 'complete', 'blocked', 'failed', 'interrupted', 'stopped')),
    reason TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL
);
CREATE TABLE execution_generations (
    project_id TEXT NOT NULL REFERENCES projects(id),
    issue_number INTEGER NOT NULL CHECK (issue_number > 0),
    generation INTEGER NOT NULL CHECK (generation > 0),
    state TEXT NOT NULL CHECK (state IN ('available', 'owned', 'succeeded', 'failed', 'blocked', 'interrupted')),
    assignment_id TEXT,
    baseline_head TEXT NOT NULL DEFAULT '',
    PRIMARY KEY (project_id, issue_number, generation)
);
CREATE UNIQUE INDEX one_open_generation ON execution_generations(project_id, issue_number)
    WHERE state IN ('available', 'owned', 'failed', 'blocked', 'interrupted');
CREATE TABLE assignments (
    id TEXT PRIMARY KEY,
    run_id TEXT NOT NULL REFERENCES runs(id),
    project_id TEXT NOT NULL REFERENCES projects(id),
    issue_number INTEGER NOT NULL,
    generation INTEGER NOT NULL,
    attempt_number INTEGER NOT NULL DEFAULT 1,
    state TEXT NOT NULL CHECK (state IN ('pending', 'running', 'succeeded', 'failed', 'blocked', 'interrupted')),
    snapshot_json TEXT NOT NULL,
    checks_receipt TEXT NOT NULL DEFAULT '',
    delivery_receipt TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL,
    settled_at TEXT,
    UNIQUE (project_id, issue_number, generation),
    FOREIGN KEY (project_id, issue_number, generation) REFERENCES execution_generations(project_id, issue_number, generation)
);
CREATE UNIQUE INDEX one_unsettled_assignment_per_run ON assignments(run_id)
    WHERE state IN ('pending', 'running', 'failed', 'blocked', 'interrupted');
CREATE TABLE assignment_attempts (
    id TEXT PRIMARY KEY,
    assignment_id TEXT NOT NULL REFERENCES assignments(id),
    ordinal INTEGER NOT NULL CHECK (ordinal > 0),
    phase TEXT NOT NULL CHECK (phase IN ('preparing', 'implementing', 'validating', 'repairing', 'closing', 'delivering', 'awaiting_review', 'settled')),
    outcome TEXT NOT NULL CHECK (outcome IN ('pending', 'running', 'succeeded', 'blocked', 'failed', 'interrupted', 'canceled')),
    predecessor_id TEXT REFERENCES assignment_attempts(id),
    created_at TEXT NOT NULL,
    settled_at TEXT,
    UNIQUE (assignment_id, ordinal)
);
CREATE UNIQUE INDEX one_open_attempt ON assignment_attempts(assignment_id)
    WHERE outcome IN ('pending', 'running');

-- +goose Down
DROP INDEX one_open_attempt;
DROP TABLE assignment_attempts;
DROP INDEX one_unsettled_assignment_per_run;
DROP TABLE assignments;
DROP INDEX one_open_generation;
DROP TABLE execution_generations;
DROP TABLE run_schedules;
DROP INDEX one_active_run;
CREATE UNIQUE INDEX one_active_run ON runs((1)) WHERE state IN ('reserved', 'running', 'interrupted');
ALTER TABLE runs DROP COLUMN reservation_state;
