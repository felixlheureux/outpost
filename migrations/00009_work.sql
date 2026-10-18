-- +goose Up
DROP TRIGGER activity_provider_insert;
DROP TRIGGER activity_provider_update;
ALTER TABLE provider_attempts RENAME TO provider_attempts_old;
CREATE TABLE provider_attempts (
    attempt_id TEXT PRIMARY KEY REFERENCES assignment_attempts(id),
    assignment_id TEXT NOT NULL REFERENCES assignments(id),
    state TEXT NOT NULL CHECK (state IN ('pending','thread_started','turn_started','completed','failed','interrupted','unknown')),
    version TEXT NOT NULL, model TEXT NOT NULL, reasoning TEXT NOT NULL,
    sandbox TEXT NOT NULL, approval_policy TEXT NOT NULL, prompt TEXT NOT NULL,
    thread_id TEXT NOT NULL DEFAULT '', turn_id TEXT NOT NULL DEFAULT '',
    handoff_json TEXT NOT NULL DEFAULT '', failure_code TEXT NOT NULL DEFAULT '',
    turn_open INTEGER NOT NULL DEFAULT 0 CHECK(turn_open IN (0,1)),
    created_at TEXT NOT NULL, updated_at TEXT NOT NULL
);
INSERT INTO provider_attempts(attempt_id,assignment_id,state,version,model,reasoning,sandbox,approval_policy,prompt,thread_id,turn_id,handoff_json,failure_code,created_at,updated_at,turn_open)
SELECT attempt_id,assignment_id,state,version,model,reasoning,sandbox,approval_policy,prompt,thread_id,turn_id,handoff_json,failure_code,created_at,updated_at,CASE WHEN state='turn_started' THEN 1 ELSE 0 END FROM provider_attempts_old;
DROP TABLE provider_attempts_old;
CREATE INDEX provider_assignment_latest ON provider_attempts(assignment_id,created_at DESC);
-- +goose StatementBegin
CREATE TRIGGER activity_provider_insert AFTER INSERT ON provider_attempts BEGIN
    INSERT INTO activity_events(project_id,run_id,attempt_id,kind,payload_json,created_at)
    SELECT a.project_id,a.run_id,NEW.attempt_id,'activity.appended',json_object('category','provider','state',NEW.state,'model',NEW.model,'reasoning',NEW.reasoning),strftime('%Y-%m-%dT%H:%M:%fZ','now') FROM assignments a WHERE a.id=NEW.assignment_id;
END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER activity_provider_update AFTER UPDATE OF state ON provider_attempts BEGIN
    INSERT INTO activity_events(project_id,run_id,attempt_id,kind,payload_json,created_at)
    SELECT a.project_id,a.run_id,NEW.attempt_id,'activity.appended',json_object('category','provider','state',NEW.state,'thread_id',NEW.thread_id,'turn_id',NEW.turn_id,'failure_code',NEW.failure_code),strftime('%Y-%m-%dT%H:%M:%fZ','now') FROM assignments a WHERE a.id=NEW.assignment_id;
END;
-- +goose StatementEnd
ALTER TABLE activity_artifacts RENAME TO activity_artifacts_old;
CREATE TABLE activity_artifacts (
    id TEXT PRIMARY KEY,
    run_id TEXT NOT NULL REFERENCES runs(id),
    attempt_id TEXT NOT NULL UNIQUE REFERENCES assignment_attempts(id),
    assignment_id TEXT NOT NULL REFERENCES assignments(id),
    kind TEXT NOT NULL CHECK (kind IN ('provider-events')),
    relative_path TEXT NOT NULL UNIQUE,
    bytes_written INTEGER NOT NULL DEFAULT 0 CHECK(bytes_written >= 0),
    truncated INTEGER NOT NULL DEFAULT 0 CHECK(truncated IN (0,1)),
    retention_state TEXT NOT NULL DEFAULT 'retained' CHECK(retention_state IN ('retained','pending','removed','unknown')),
    removed_at TEXT NOT NULL DEFAULT '', created_at TEXT NOT NULL, updated_at TEXT NOT NULL
);
INSERT INTO activity_artifacts SELECT * FROM activity_artifacts_old;
DROP TABLE activity_artifacts_old;
CREATE TABLE work_messages (
    id TEXT PRIMARY KEY,
    run_id TEXT NOT NULL REFERENCES runs(id),
    assignment_id TEXT NOT NULL REFERENCES assignments(id),
    attempt_id TEXT NOT NULL REFERENCES assignment_attempts(id),
    thread_id TEXT NOT NULL, expected_turn_id TEXT NOT NULL,
    target TEXT NOT NULL CHECK(target IN ('live','next')),
    body TEXT NOT NULL, digest TEXT NOT NULL,
    state TEXT NOT NULL CHECK(state IN ('queued','sending','accepted','rejected','unknown')),
    reason TEXT NOT NULL DEFAULT '', contract_changed INTEGER NOT NULL DEFAULT 1,
    created_at TEXT NOT NULL, updated_at TEXT NOT NULL
);
CREATE INDEX work_messages_attempt ON work_messages(attempt_id,created_at);
-- +goose StatementBegin
CREATE TRIGGER activity_work_message_insert AFTER INSERT ON work_messages BEGIN
    INSERT INTO activity_events(project_id,run_id,attempt_id,kind,payload_json,created_at)
    SELECT a.project_id,NEW.run_id,NEW.attempt_id,'message.changed',json_object('id',NEW.id,'state',NEW.state,'target',NEW.target),strftime('%Y-%m-%dT%H:%M:%fZ','now') FROM assignments a WHERE a.id=NEW.assignment_id;
END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER activity_work_message_update AFTER UPDATE OF state ON work_messages BEGIN
    INSERT INTO activity_events(project_id,run_id,attempt_id,kind,payload_json,created_at)
    SELECT a.project_id,NEW.run_id,NEW.attempt_id,'message.changed',json_object('id',NEW.id,'state',NEW.state,'target',NEW.target,'reason',NEW.reason,'contract_changed',NEW.contract_changed),strftime('%Y-%m-%dT%H:%M:%fZ','now') FROM assignments a WHERE a.id=NEW.assignment_id;
END;
-- +goose StatementEnd
CREATE TABLE work_controls (
    id TEXT PRIMARY KEY, run_id TEXT NOT NULL REFERENCES runs(id),
    attempt_id TEXT NOT NULL REFERENCES assignment_attempts(id),
    action TEXT NOT NULL CHECK(action IN ('finish','undo','interrupt','continue')),
    mode TEXT NOT NULL DEFAULT '', instructions TEXT NOT NULL DEFAULT '', digest TEXT NOT NULL,
    state TEXT NOT NULL CHECK(state IN ('queued','sending','accepted','rejected','unknown')),
    reason TEXT NOT NULL DEFAULT '', created_at TEXT NOT NULL, updated_at TEXT NOT NULL
);
CREATE INDEX work_controls_run ON work_controls(run_id,created_at);
-- +goose StatementBegin
CREATE TRIGGER activity_work_control_insert AFTER INSERT ON work_controls BEGIN
    INSERT INTO activity_events(project_id,run_id,attempt_id,kind,payload_json,created_at)
    SELECT r.project_id,NEW.run_id,NEW.attempt_id,'run.changed',json_object('control',NEW.action,'state',NEW.state,'id',NEW.id),strftime('%Y-%m-%dT%H:%M:%fZ','now') FROM runs r WHERE r.id=NEW.run_id;
END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER activity_work_control_update AFTER UPDATE OF state ON work_controls BEGIN
    INSERT INTO activity_events(project_id,run_id,attempt_id,kind,payload_json,created_at)
    SELECT r.project_id,NEW.run_id,NEW.attempt_id,'run.changed',json_object('control',NEW.action,'state',NEW.state,'id',NEW.id,'reason',NEW.reason),strftime('%Y-%m-%dT%H:%M:%fZ','now') FROM runs r WHERE r.id=NEW.run_id;
END;
-- +goose StatementEnd
-- +goose Down
-- Linked continuation history cannot be represented in schema 8. Keep data intact.
SELECT cannot_downgrade_work_history_without_data_loss FROM outpost_manual_downgrade_required;
