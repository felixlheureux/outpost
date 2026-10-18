# Scheduler handoff

The worker claims its recorded systemd identity and workspace advisory lock
before invoking `scheduler.Engine.Drive`. `Drive` fetches the planning manifest
and every referenced issue/dependency outside a database transaction, walks the
manifest in order, and atomically claims one assignment using the run revision,
current local manifest SHA, and issue digests. The recorded fetch time, SHA,
issue digest/revision, queue position, branch, HEAD, Git status, instructions,
policy, and dependency heads are the assignment's cutover. A GitHub edit after
that observation affects a later boundary; no cross-service transaction exists.

The worker callback receives a pinned assignment. It must preserve uncertain
provider/check/delivery outcomes rather than return success. A successful
settlement requires a verified checkout HEAD plus distinct check and delivery
receipt IDs recorded in the validating/delivering phases. OP-011/012 must
supply real receipts; fixture IDs in scheduler tests establish ordering, stop
races, and budgets only.

`run_schedules.revision` guards stop, undo, admission, and final settlement.
The global `runs.reservation_state` is independent of the display state.
The supervisor releases it only after observing the exact worker unit stopped
and acquiring the workspace lock, then records an interrupted unfinished
assignment in the same transaction. Failed/interrupted generations remain
assigned to their project and block another task in that checkout. Another
project can reserve the global slot after safe release; project switching alone
never changes a run's recorded project.

`POST /api/v1/projects/:id/admission-preview` accepts `{"mode":"follow"}` or
`{"mode":"fixed","selected":41}`. It refreshes GitHub and returns selected
issue, waiting codes/explanations, queue SHA, fetch time, and an outcome such as
`drained`, `blocked`, `source_unavailable`, or `baseline_changed`. It is an
inspection operation; it never reserves or launches a worker. The UI keeps
Start disabled until OP-008 can show actual provider and policy readiness.

Source outages retain the prior projection but cannot authorize admission.
A completed generation is skipped even while reporting is pending or its issue
body changes. Reopening a completed generation requires the explicit guarded
`ReopenGeneration` operation; automatic source refresh cannot create one.
