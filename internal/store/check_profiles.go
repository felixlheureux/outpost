package store

import (
	"context"
	"fmt"
	"time"
)

func (s *Store) SetCheckProfile(ctx context.Context, projectID, id, profileJSON string, makeDefault bool) error {
	if projectID == "" || id == "" || len(profileJSON) > 64<<10 {
		return fmt.Errorf("invalid check profile")
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	_, err = tx.ExecContext(ctx, `INSERT INTO project_check_profiles(project_id,id,profile_json,configured_at) VALUES(?,?,?,?) ON CONFLICT(project_id,id) DO UPDATE SET profile_json=excluded.profile_json,configured_at=excluded.configured_at`, projectID, id, profileJSON, time.Now().UTC().Format(time.RFC3339Nano))
	if err != nil {
		return err
	}
	if makeDefault {
		_, err = tx.ExecContext(ctx, `INSERT INTO project_check_defaults(project_id,profile_id) VALUES(?,?) ON CONFLICT(project_id) DO UPDATE SET profile_id=excluded.profile_id`, projectID, id)
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}
func (s *Store) GetCheckProfile(ctx context.Context, projectID, id string) (string, error) {
	var raw string
	err := s.DB.QueryRowContext(ctx, `SELECT profile_json FROM project_check_profiles WHERE project_id=? AND id=?`, projectID, id).Scan(&raw)
	return raw, err
}
func (s *Store) DefaultCheckProfile(ctx context.Context, projectID string) (string, error) {
	var raw string
	err := s.DB.QueryRowContext(ctx, `SELECT p.profile_json FROM project_check_defaults d JOIN project_check_profiles p ON p.project_id=d.project_id AND p.id=d.profile_id WHERE d.project_id=?`, projectID).Scan(&raw)
	return raw, err
}
