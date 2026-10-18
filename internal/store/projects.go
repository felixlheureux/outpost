package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

type ProjectSetup struct {
	ID, Owner, Name, RepositoryURL, WorkspacePath string
	GithubID                                      int64
	State, Stage, Error                           string
	Managed                                       bool
}

func scanProject(row *sql.Row) (ProjectSetup, error) {
	var p ProjectSetup
	var workspace, setupError sql.NullString
	err := row.Scan(&p.ID, &p.Owner, &p.Name, &p.RepositoryURL, &p.GithubID, &workspace, &p.State, &p.Stage, &setupError, &p.Managed)
	p.WorkspacePath, p.Error = workspace.String, setupError.String
	return p, err
}

const projectColumns = "id, github_owner, github_name, repository_url, github_id, workspace_path, setup_state, setup_stage, setup_error, managed"

func (s *Store) GetProjectSetup(ctx context.Context, id string) (ProjectSetup, error) {
	return scanProject(s.DB.QueryRowContext(ctx, "SELECT "+projectColumns+" FROM projects WHERE id = ?", id))
}

// ReserveProject records GitHub's stable identity before any clone. A repeat
// URL or concurrent request receives the same project/setup identity.
func (s *Store) ReserveProject(ctx context.Context, githubID int64, owner, name, workspacePath string) (ProjectSetup, bool, error) {
	return s.reserveProject(ctx, githubID, owner, name, workspacePath, true)
}

func (s *Store) ReserveExistingProject(ctx context.Context, githubID int64, owner, name, workspacePath string) (ProjectSetup, bool, error) {
	return s.reserveProject(ctx, githubID, owner, name, workspacePath, false)
}

func (s *Store) reserveProject(ctx context.Context, githubID int64, owner, name, workspacePath string, managed bool) (ProjectSetup, bool, error) {
	if githubID <= 0 || owner == "" || name == "" || workspacePath == "" {
		return ProjectSetup{}, false, fmt.Errorf("incomplete repository identity")
	}
	id := fmt.Sprintf("project_%d", githubID)
	url := "https://github.com/" + owner + "/" + name
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return ProjectSetup{}, false, err
	}
	defer tx.Rollback()
	var pathOwner string
	err = tx.QueryRowContext(ctx, "SELECT id FROM projects WHERE workspace_path = ? AND id != ?", workspacePath, id).Scan(&pathOwner)
	if err == nil {
		return ProjectSetup{}, false, fmt.Errorf("checkout is already owned by another registered repository")
	}
	if err != sql.ErrNoRows {
		return ProjectSetup{}, false, err
	}
	var existingID string
	err = tx.QueryRowContext(ctx, "SELECT id FROM projects WHERE github_id = ? OR (github_owner = ? COLLATE NOCASE AND github_name = ? COLLATE NOCASE)", githubID, owner, name).Scan(&existingID)
	if err == nil {
		p, readErr := scanProject(tx.QueryRowContext(ctx, "SELECT "+projectColumns+" FROM projects WHERE id = ?", existingID))
		if readErr != nil {
			return ProjectSetup{}, false, readErr
		}
		if p.GithubID != githubID {
			return ProjectSetup{}, false, fmt.Errorf("repository name is already registered under another GitHub identity")
		}
		if p.WorkspacePath != workspacePath {
			return ProjectSetup{}, false, fmt.Errorf("repository is already registered with a different checkout")
		}
		return p, true, tx.Commit()
	}
	if err != sql.ErrNoRows {
		return ProjectSetup{}, false, err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err = tx.ExecContext(ctx, `INSERT INTO projects(id, github_owner, github_name, repository_url, setup_state, created_at, github_id, workspace_path, setup_stage, managed) VALUES (?, ?, ?, ?, 'pending', ?, ?, ?, 'reserved', ?)`, id, owner, name, url, now, githubID, workspacePath, managed)
	if err != nil {
		return ProjectSetup{}, false, err
	}
	if err := tx.Commit(); err != nil {
		return ProjectSetup{}, false, err
	}
	return ProjectSetup{ID: id, Owner: owner, Name: name, RepositoryURL: url, GithubID: githubID, WorkspacePath: workspacePath, State: "pending", Stage: "reserved", Managed: managed}, false, nil
}

func (s *Store) SetProjectStage(ctx context.Context, id, stage, state, message string) error {
	if state != "pending" && state != "ready" && state != "failed" {
		return fmt.Errorf("invalid setup state")
	}
	if len(message) > 300 || strings.ContainsAny(message, "\r\n") {
		return fmt.Errorf("invalid setup message")
	}
	result, err := s.DB.ExecContext(ctx, "UPDATE projects SET setup_stage = ?, setup_state = ?, setup_error = ? WHERE id = ?", stage, state, message, id)
	if err != nil {
		return err
	}
	if count, _ := result.RowsAffected(); count != 1 {
		return fmt.Errorf("project setup was not found")
	}
	return nil
}
