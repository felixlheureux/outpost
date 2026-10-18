package scheduler

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/felixlheureux/outpost/internal/store"
)

// Execute consumes one pinned assignment. An error means its effect may be
// uncertain, so Drive retains the assignment and stops without admitting more.
type Execute func(context.Context, store.Assignment) (outcome, verifiedHead string, err error)

// Drive is called by the claimed worker while it owns the workspace lock.
// OP-008 supplies the real provider execution callback. Fixtures can exercise
// the same admission and stop boundary without pretending to run Codex.
func (e Engine) Drive(ctx context.Context, runID string, execute Execute) (Projection, error) {
	for {
		before, err := e.DB.GetSchedule(ctx, runID)
		if err != nil {
			return Projection{}, err
		}
		p, a, err := e.Next(ctx, runID, true)
		if errors.Is(err, store.ErrAdmissionChanged) {
			continue
		}
		if err != nil {
			return p, err
		}
		if a == nil {
			if p.Outcome == "unfinished_assignment" || before.Outcome != "active" {
				return p, nil
			}
			outcome := "blocked"
			switch p.Outcome {
			case "drained", "budget_reached", "externally_closed":
				outcome = "complete"
			case "stopped":
				outcome = "stopped"
			}
			reasons := make([]string, 0, len(p.Waiting)+1)
			reasons = append(reasons, p.Outcome)
			for _, w := range p.Waiting {
				if len(reasons) >= 6 {
					break
				}
				reasons = append(reasons, fmt.Sprintf("#%d %s: %s", w.Issue, w.Code, w.Reason))
			}
			if err = e.DB.SettleSchedule(ctx, runID, before.Revision, outcome, strings.Join(reasons, "; ")); errors.Is(err, store.ErrAdmissionChanged) {
				continue
			} else if err != nil {
				return p, err
			}
			return p, nil
		}
		if err := e.DB.BeginAttempt(ctx, a.ID); err != nil {
			return p, err
		}
		result, head, err := execute(ctx, *a)
		if err != nil {
			return p, err
		}
		if err = e.DB.SettleAssignment(ctx, a.ID, result, head); err != nil {
			return p, err
		}
		if result != "succeeded" {
			return p, nil
		}
	}
}
