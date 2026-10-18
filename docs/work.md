# Work controls and receipts

The Work page reads `GET /api/v1/runs/:id/work` for the current pinned assignment,
latest linked attempt, provider IDs, run budget, stop state, and recent message and
control receipts. The activity SSE invalidates this view; the page also polls to
recover after a disconnected stream. `GET /api/v1/runs/:id/changes` reads the owned
checkout's current Git status. A `path` query returns one bounded text diff or
untracked file preview on demand. These are observations of the checkout, not
verified task completion.

`POST /api/v1/runs/:id/messages` requires a client ID, attempt ID, thread ID,
target (`live` or `next`), instructions, and the expected turn ID for a live
target. The SQLite transaction checks the active assignment, provider session,
and open turn. The ID and payload digest make retries idempotent; changed payload
with the same ID conflicts. The worker claims `queued` messages as `sending`,
calls Codex `turn/steer`, then records `accepted`, `rejected`, or `unknown` from
the provider acknowledgement. A terminal turn rejects pending live messages.
Sending messages with lost acknowledgement become `unknown` and are not replayed.
`contract_changed` on an accepted receipt tells the future validation owner to
include current operator direction in its contract identity. It does not mean
the provider implemented the instruction.

`POST /api/v1/runs/:id/controls` records `finish`, `undo`, `interrupt`, or
`continue`. Finish and undo update the schedule revision in the same transaction
as the receipt; the next admission checks that revision and stop state. Interrupt
sets the stop guard before the worker calls `turn/interrupt`. The worker waits for
the terminal turn and closes the owned App Server process group. Undo cannot
cancel an immediate interruption. Continue requires explicit `resume` or `fresh`
mode and creates a new attempt linked to its predecessor only after the old
provider process has closed. Resume requires a recorded thread ID and exact
workspace identity. The original assignment snapshot and earlier provider prompt
remain immutable; the new prompt includes the recorded continuation directions.
The successful task count and limit remain on the same run. An `accepted`
continuation receipt means the linked attempt was created; it does not mean the
task completed. A successful saved-session or fresh continuation proceeds through
the same runner-owned checks, selected Git delivery, and assignment settlement as
an uninterrupted turn. Completion still requires check and delivery receipts; see
`docs/validation.md` and `docs/delivery.md`.

An immediate interrupt retains its stop request through continuation. After the
continued assignment settles, this run stops before another queue admission. The
next task can start in a new run once the execution slot is released. At other
Follow queue assignment boundaries, the runner refreshes and uses the latest
valid queue order and pending issue definitions; the active assignment remains
pinned.

The one browser composer stores its unsent text in tab scoped session storage
keyed by project and attempt. An HTTP submission does not clear the text. The
current message receipt must reach `accepted` before that draft is cleared;
unknown or rejected submissions remain visible for inspection. Continuation
starts a new scoped draft rather than silently retargeting the old one.

Schema 9 retains each provider attempt and artifact separately. Migration from
schema 8 preserves the old provider receipt and artifact ID. Schema 9 cannot be
downgraded to 8 without losing linked continuation history.
