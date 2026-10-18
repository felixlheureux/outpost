package projectsetup

import (
	"context"

	"github.com/felixlheureux/outpost/internal/store"
)

// Retry uses the persisted repository and checkout identity. It never chooses a
// new destination from request input, and Setup rechecks owned staging work.
func Retry(ctx context.Context, db *store.Store, stateDir, workspacesDir, projectID string) (store.ProjectSetup, error) {
	project, err := db.GetProjectSetup(ctx, projectID)
	if err != nil {
		return store.ProjectSetup{}, err
	}
	if project.Managed {
		return Setup(ctx, db, stateDir, workspacesDir, project.RepositoryURL)
	}
	return RegisterExisting(ctx, db, project.WorkspacePath)
}
