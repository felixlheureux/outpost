-- +goose Up
CREATE TABLE activity_events (
    cursor INTEGER PRIMARY KEY AUTOINCREMENT,
    project_id TEXT REFERENCES projects(id),
    run_id TEXT REFERENCES runs(id),
    attempt_id TEXT REFERENCES assignment_attempts(id),
    kind TEXT NOT NULL CHECK (kind IN ('run.changed','task.changed','attempt.changed','phase.changed','activity.appended','check.changed','message.changed','delivery.changed','source.changed')),
    payload_json TEXT NOT NULL CHECK (length(payload_json) <= 65536 AND json_valid(payload_json)),
    artifact_id TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL
);
CREATE INDEX activity_run_cursor ON activity_events(run_id,cursor);
CREATE TABLE activity_cursor_floor (
    run_id TEXT PRIMARY KEY REFERENCES runs(id),
    pruned_through INTEGER NOT NULL DEFAULT 0
);
-- +goose StatementBegin
CREATE TRIGGER activity_deleted AFTER DELETE ON activity_events WHEN OLD.run_id IS NOT NULL BEGIN
    INSERT INTO activity_cursor_floor(run_id,pruned_through) VALUES(OLD.run_id,OLD.cursor)
    ON CONFLICT(run_id) DO UPDATE SET pruned_through=max(pruned_through,excluded.pruned_through);
END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER activity_retention AFTER INSERT ON activity_events BEGIN
    DELETE FROM activity_events WHERE cursor <= NEW.cursor - 10000;
END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER activity_run_insert AFTER INSERT ON runs BEGIN
    INSERT INTO activity_events(project_id,run_id,kind,payload_json,created_at)
    VALUES(NEW.project_id,NEW.id,'run.changed',json_object('run_id',NEW.id,'state',NEW.state,'reservation_state',NEW.reservation_state),strftime('%Y-%m-%dT%H:%M:%fZ','now'));
END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER activity_run_update AFTER UPDATE OF state,reservation_state,worker_pid,launch_observation ON runs BEGIN
    INSERT INTO activity_events(project_id,run_id,kind,payload_json,created_at)
    VALUES(NEW.project_id,NEW.id,'run.changed',json_object('run_id',NEW.id,'state',NEW.state,'reservation_state',NEW.reservation_state,'launch_observation',NEW.launch_observation),strftime('%Y-%m-%dT%H:%M:%fZ','now'));
END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER activity_assignment_insert AFTER INSERT ON assignments BEGIN
    INSERT INTO activity_events(project_id,run_id,kind,payload_json,created_at)
    VALUES(NEW.project_id,NEW.run_id,'task.changed',json_object('assignment_id',NEW.id,'issue_number',NEW.issue_number,'state',NEW.state),strftime('%Y-%m-%dT%H:%M:%fZ','now'));
END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER activity_assignment_update AFTER UPDATE OF state ON assignments BEGIN
    INSERT INTO activity_events(project_id,run_id,kind,payload_json,created_at)
    VALUES(NEW.project_id,NEW.run_id,'task.changed',json_object('assignment_id',NEW.id,'issue_number',NEW.issue_number,'state',NEW.state),strftime('%Y-%m-%dT%H:%M:%fZ','now'));
END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER activity_attempt_insert AFTER INSERT ON assignment_attempts BEGIN
    INSERT INTO activity_events(project_id,run_id,attempt_id,kind,payload_json,created_at)
    SELECT a.project_id,a.run_id,NEW.id,'attempt.changed',json_object('assignment_id',NEW.assignment_id,'attempt_id',NEW.id,'phase',NEW.phase,'outcome',NEW.outcome),strftime('%Y-%m-%dT%H:%M:%fZ','now') FROM assignments a WHERE a.id=NEW.assignment_id;
END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER activity_attempt_update AFTER UPDATE OF phase,outcome ON assignment_attempts BEGIN
    INSERT INTO activity_events(project_id,run_id,attempt_id,kind,payload_json,created_at)
    SELECT a.project_id,a.run_id,NEW.id,'phase.changed',json_object('assignment_id',NEW.assignment_id,'attempt_id',NEW.id,'phase',NEW.phase,'outcome',NEW.outcome),strftime('%Y-%m-%dT%H:%M:%fZ','now') FROM assignments a WHERE a.id=NEW.assignment_id;
END;
-- +goose StatementEnd
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
-- +goose StatementBegin
CREATE TRIGGER activity_issue_source_insert AFTER INSERT ON issue_source_status BEGIN
    INSERT INTO activity_events(project_id,kind,payload_json,created_at)
    VALUES(NEW.project_id,'source.changed',json_object('source','issues','state',NEW.state),strftime('%Y-%m-%dT%H:%M:%fZ','now'));
END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER activity_issue_source_update AFTER UPDATE OF state,last_success_at ON issue_source_status BEGIN
    INSERT INTO activity_events(project_id,kind,payload_json,created_at)
    VALUES(NEW.project_id,'source.changed',json_object('source','issues','state',NEW.state),strftime('%Y-%m-%dT%H:%M:%fZ','now'));
END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER activity_queue_source_insert AFTER INSERT ON queue_sources BEGIN
    INSERT INTO activity_events(project_id,kind,payload_json,created_at)
    VALUES(NEW.project_id,'source.changed',json_object('source','queue','state',NEW.state),strftime('%Y-%m-%dT%H:%M:%fZ','now'));
END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER activity_queue_source_update AFTER UPDATE OF state,blob_sha ON queue_sources BEGIN
    INSERT INTO activity_events(project_id,kind,payload_json,created_at)
    VALUES(NEW.project_id,'source.changed',json_object('source','queue','state',NEW.state),strftime('%Y-%m-%dT%H:%M:%fZ','now'));
END;
-- +goose StatementEnd
CREATE TABLE activity_artifacts (
    id TEXT PRIMARY KEY,
    run_id TEXT NOT NULL REFERENCES runs(id),
    attempt_id TEXT NOT NULL REFERENCES assignment_attempts(id),
    assignment_id TEXT NOT NULL UNIQUE REFERENCES assignments(id),
    kind TEXT NOT NULL CHECK (kind IN ('provider-events')),
    relative_path TEXT NOT NULL UNIQUE,
    bytes_written INTEGER NOT NULL DEFAULT 0 CHECK(bytes_written >= 0),
    truncated INTEGER NOT NULL DEFAULT 0 CHECK(truncated IN (0,1)),
    retention_state TEXT NOT NULL DEFAULT 'retained' CHECK(retention_state IN ('retained','pending','removed','unknown')),
    removed_at TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);
-- +goose Down
DROP TABLE activity_artifacts;
DROP TRIGGER activity_queue_source_update;
DROP TRIGGER activity_queue_source_insert;
DROP TRIGGER activity_issue_source_update;
DROP TRIGGER activity_issue_source_insert;
DROP TRIGGER activity_provider_update;
DROP TRIGGER activity_provider_insert;
DROP TRIGGER activity_attempt_update;
DROP TRIGGER activity_attempt_insert;
DROP TRIGGER activity_assignment_update;
DROP TRIGGER activity_assignment_insert;
DROP TRIGGER activity_run_update;
DROP TRIGGER activity_run_insert;
DROP TRIGGER activity_retention;
DROP TRIGGER activity_deleted;
DROP TABLE activity_cursor_floor;
DROP INDEX activity_run_cursor;
DROP TABLE activity_events;
