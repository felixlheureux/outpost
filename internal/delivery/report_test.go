package delivery

import (
	"context"
	"fmt"
	"testing"

	"github.com/felixlheureux/outpost/internal/githubcli"
)

type reportFixture struct {
	comments      []Comment
	posts, closes int
	losePost      bool
	closed        bool
}

func (f *reportFixture) Comments(_ context.Context, _, _ string, _ int) ([]Comment, error) {
	return f.comments, nil
}
func (f *reportFixture) Post(_ context.Context, _, _ string, _ int, body string) (Comment, error) {
	f.posts++
	c := Comment{ID: int64(f.posts), Body: body}
	f.comments = append(f.comments, c)
	if f.losePost {
		return Comment{}, fmt.Errorf("lost acknowledgement")
	}
	return c, nil
}
func (f *reportFixture) Issue(_ context.Context, _, _ string, _ int) (githubcli.Issue, error) {
	state := "open"
	if f.closed {
		state = "closed"
	}
	return githubcli.Issue{State: state}, nil
}
func (f *reportFixture) Close(_ context.Context, _, _ string, _ int) error {
	f.closes++
	f.closed = true
	return nil
}

func TestUnknownCommentReconcilesWithoutDuplicate(t *testing.T) {
	ctx := context.Background()
	db, dir, a, s, candidate := setupDelivery(t, "commit")
	if _, err := Deliver(ctx, db, dir, a, s, candidate); err != nil {
		t.Fatal(err)
	}
	if _, err := db.DB.ExecContext(ctx, `UPDATE assignments SET state='succeeded' WHERE id=?`, a.ID); err != nil {
		t.Fatal(err)
	}
	f := &reportFixture{losePost: true}
	if _, err := Report(ctx, db, a, true, f); err == nil {
		t.Fatal("lost acknowledgement should remain unknown")
	}
	f.losePost = false
	r, err := Report(ctx, db, a, true, f)
	if err != nil {
		t.Fatal(err)
	}
	if r.State != "settled" || f.posts != 1 || f.closes != 1 {
		t.Fatalf("report=%+v posts=%d closes=%d", r, f.posts, f.closes)
	}
	if _, err = Report(ctx, db, a, true, f); err != nil || f.posts != 1 || f.closes != 1 {
		t.Fatalf("replay: %v posts=%d closes=%d", err, f.posts, f.closes)
	}
}
