-- +goose Up
ALTER TABLE projects ADD COLUMN github_id INTEGER;
ALTER TABLE projects ADD COLUMN workspace_path TEXT;
ALTER TABLE projects ADD COLUMN setup_stage TEXT NOT NULL DEFAULT 'pending';
ALTER TABLE projects ADD COLUMN setup_error TEXT;
ALTER TABLE projects ADD COLUMN managed INTEGER NOT NULL DEFAULT 0;
CREATE UNIQUE INDEX projects_github_id_unique ON projects(github_id) WHERE github_id IS NOT NULL;

-- +goose Down
DROP INDEX projects_github_id_unique;
ALTER TABLE projects DROP COLUMN managed;
ALTER TABLE projects DROP COLUMN setup_error;
ALTER TABLE projects DROP COLUMN setup_stage;
ALTER TABLE projects DROP COLUMN workspace_path;
ALTER TABLE projects DROP COLUMN github_id;
