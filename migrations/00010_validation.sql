-- +goose Up
CREATE TABLE project_check_profiles (
    project_id TEXT NOT NULL REFERENCES projects(id),
    id TEXT NOT NULL,
    profile_json TEXT NOT NULL CHECK(json_valid(profile_json)),
    configured_at TEXT NOT NULL,
    PRIMARY KEY(project_id,id)
);
CREATE TABLE project_check_defaults (
    project_id TEXT PRIMARY KEY REFERENCES projects(id),
    profile_id TEXT NOT NULL,
    FOREIGN KEY(project_id,profile_id) REFERENCES project_check_profiles(project_id,id)
);
CREATE TABLE check_receipts (
    id TEXT PRIMARY KEY,
    assignment_id TEXT NOT NULL REFERENCES assignments(id),
    attempt_id TEXT NOT NULL REFERENCES assignment_attempts(id),
    generation INTEGER NOT NULL,
    check_id TEXT NOT NULL,
    name TEXT NOT NULL,
    required INTEGER NOT NULL CHECK(required IN (0,1)),
    command_json TEXT NOT NULL,
    env_json TEXT NOT NULL DEFAULT '{}',
    contract_digest TEXT NOT NULL,
    input_digest TEXT NOT NULL,
    candidate_digest TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL CHECK(status IN ('pending','running','passed','failed','interrupted','reused','unknown')),
    reason TEXT NOT NULL DEFAULT '',
    pid INTEGER NOT NULL DEFAULT 0,
    start_ticks TEXT NOT NULL DEFAULT '',
    started_at TEXT NOT NULL,
    finished_at TEXT NOT NULL DEFAULT '',
    duration_ms INTEGER NOT NULL DEFAULT 0,
    exit_code INTEGER,
    signal TEXT NOT NULL DEFAULT '',
    case_count INTEGER,
    require_cases INTEGER NOT NULL DEFAULT 0 CHECK(require_cases IN (0,1)),
    output_path TEXT NOT NULL,
    terminal_path TEXT NOT NULL,
    output_bytes INTEGER NOT NULL DEFAULT 0,
    truncated INTEGER NOT NULL DEFAULT 0 CHECK(truncated IN (0,1))
);
CREATE INDEX check_receipts_assignment ON check_receipts(assignment_id,check_id,started_at DESC);
-- +goose StatementBegin
CREATE TRIGGER activity_check_insert AFTER INSERT ON check_receipts BEGIN
    INSERT INTO activity_events(project_id,run_id,attempt_id,kind,payload_json,created_at)
    SELECT a.project_id,a.run_id,NEW.attempt_id,'check.changed',json_object('id',NEW.id,'check_id',NEW.check_id,'status',NEW.status),strftime('%Y-%m-%dT%H:%M:%fZ','now') FROM assignments a WHERE a.id=NEW.assignment_id;
END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER activity_check_update AFTER UPDATE OF status ON check_receipts BEGIN
    INSERT INTO activity_events(project_id,run_id,attempt_id,kind,payload_json,created_at)
    SELECT a.project_id,a.run_id,NEW.attempt_id,'check.changed',json_object('id',NEW.id,'check_id',NEW.check_id,'status',NEW.status,'reason',NEW.reason),strftime('%Y-%m-%dT%H:%M:%fZ','now') FROM assignments a WHERE a.id=NEW.assignment_id;
END;
-- +goose StatementEnd

-- +goose Down
DROP TRIGGER activity_check_update;
DROP TRIGGER activity_check_insert;
DROP TABLE check_receipts;
DROP TABLE project_check_defaults;
DROP TABLE project_check_profiles;
