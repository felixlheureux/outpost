-- +goose Up
CREATE TABLE delivery_receipts (
    id TEXT PRIMARY KEY,
    assignment_id TEXT NOT NULL UNIQUE REFERENCES assignments(id),
    policy TEXT NOT NULL CHECK(policy IN ('local_changes','commit','push')),
    candidate_digest TEXT NOT NULL,
    baseline_head TEXT NOT NULL,
    branch TEXT NOT NULL,
    upstream TEXT NOT NULL DEFAULT '',
    remote_digest TEXT NOT NULL DEFAULT '',
    paths_json TEXT NOT NULL CHECK(json_valid(paths_json)),
    commit_sha TEXT NOT NULL DEFAULT '',
    state TEXT NOT NULL CHECK(state IN ('pending','commit_unknown','push_unknown','settled','blocked')),
    reason TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);
CREATE TABLE github_reports (
    id TEXT PRIMARY KEY,
    assignment_id TEXT NOT NULL UNIQUE REFERENCES assignments(id),
    marker TEXT NOT NULL UNIQUE,
    state TEXT NOT NULL CHECK(state IN ('pending','comment_unknown','close_pending','close_unknown','settled','blocked')),
    comment_id INTEGER NOT NULL DEFAULT 0,
    close_issue INTEGER NOT NULL DEFAULT 0,
    reason TEXT NOT NULL DEFAULT '',
    updated_at TEXT NOT NULL
);
CREATE TABLE review_decisions (
    assignment_id TEXT NOT NULL REFERENCES assignments(id),
    candidate_digest TEXT NOT NULL,
    decision TEXT NOT NULL CHECK(decision IN ('approved','rejected')),
    feedback TEXT NOT NULL DEFAULT '',
    decided_at TEXT NOT NULL,
    PRIMARY KEY(assignment_id,candidate_digest)
);
-- +goose StatementBegin
CREATE TRIGGER activity_delivery_update AFTER UPDATE OF state ON delivery_receipts BEGIN
    INSERT INTO activity_events(project_id,run_id,attempt_id,kind,payload_json,created_at)
    SELECT a.project_id,a.run_id,t.id,'delivery.changed',json_object('id',NEW.id,'state',NEW.state,'policy',NEW.policy),strftime('%Y-%m-%dT%H:%M:%fZ','now')
    FROM assignments a JOIN assignment_attempts t ON t.assignment_id=a.id WHERE a.id=NEW.assignment_id ORDER BY t.ordinal DESC LIMIT 1;
END;
-- +goose StatementEnd
-- +goose Down
DROP TRIGGER activity_delivery_update;
DROP TABLE github_reports;
DROP TABLE review_decisions;
DROP TABLE delivery_receipts;
