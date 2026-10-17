-- name: ListProjects :many
SELECT id, github_owner, github_name, repository_url, setup_state, created_at
FROM projects ORDER BY created_at, id;
