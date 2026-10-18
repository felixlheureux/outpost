-- name: ListProjects :many
SELECT id, github_owner, github_name, repository_url, setup_state, setup_stage, setup_error, created_at
FROM projects ORDER BY created_at, id;
