# Shared queue contract

## Owner and location

Task requirements live in GitHub Issues. Queue membership/order lives in exactly
one `queue.json` on the configured planning branch, default `outpost/queue`.
SQLite records source observations and execution receipts; its cache is not an
independent reorderable queue.

The file at `planning-branch/queue.json` is a seed only. Normal Add repository
creates the branch and empty manifest when missing using the repository's default
branch as its base. It reuses an existing valid manifest byte for byte. A failed
source phase retains the checkout and can be retried. Do not pull/switch the
planning branch inside the coding checkout just to read or edit queue order.

## Schema v1

```json
{
  "version": 1,
  "issues": []
}
```

`version` is the supported schema version. `issues` contains distinct positive
issue numbers from the configured repository, in intended execution order. An
empty array is valid. Use actual published issue numbers; `OP-001` is not a number.
No comments, rank fields, provider IDs, local paths, tokens, or completion history
belong in this file. Unsupported/malformed input blocks new admission explicitly.

For illustration only, `[53,41,72]` means consider issue 53 before 41 before 72.
Those numbers are examples, not issues created by this package. Dependencies and
holds still constrain eligibility. If 72 depends on 41 and the list is
`[72,41,88]`, 41 is next when ready; 72 stays in its chosen waiting position.

## Editing safely

1. Read the current file from the configured branch through GitHub's Contents API.
2. Retain its repository/path/branch and returned blob SHA.
3. Apply a deliberate append/remove/move to the list. Preserve unrelated entries.
4. Validate numbers, uniqueness, schema, and references.
5. Submit the entire replacement with that SHA.
6. On conflict, read the new order and reconcile the intended edit. Do not force
   the old full list over it. If the response is uncertain, inspect the remote
   result before replaying the effect.

A reorder changes one file. Do not update issue titles/bodies to maintain ranks.
GitHub issue edits have different concurrency semantics; do not assume an issue
PATCH is protected by this file SHA.

## Local API and receipts

`GET /api/v1/projects/:id/queue` returns the cached SHA, ordered issue numbers,
fetch time and source health. `POST .../queue/sync` refreshes the manifest and
checks each referenced issue. The visible Queue view polls the cache every 30
seconds and offers manual Sync. A failed refresh keeps the last good order and
sets source health to `needs_attention`; it does not authorize new admission.

`POST .../queue/operations` accepts a stable `request_key`, `expected_sha`, and
one `kind`: `replace` with `issues`, `append` or `remove` with `issue`, or `move`
with `issue` and optional `before` neighbor (omitted means end). It returns a
receipt with `pending`, `confirmed`, `rejected`, `conflict`, or `unknown`, plus the
intended edit and observed current order. `GET .../queue/operations` lists recent
receipts; `GET .../queue/operations/:key` reads one; `POST
.../queue/operations/:key/inspect` checks an uncertain write without replaying
it. Reuse a key only for the exact same edit. A stale SHA yields a conflict and
requires a new deliberate edit against the freshly observed order.

The edit intent and calculated replacement are stored before the GitHub PUT.
After a write, inspection checks the current blob and latest path commit marker.
If either cannot prove the effect, the receipt stays `unknown`. A later writer
may replace an earlier confirmed order; the latest manifest remains authoritative.
The Contents API SHA check prevents routine stale overwrites but does not create
a distributed execution lock across installations.

## Execution behavior

Follow queue is the default. The worker refreshes the source before admitting
each task, captures an immutable assignment, and applies later source changes at
the next boundary. Adding/moving/removing pending work never preempts active work.
Fixed scope is an explicitly selected alternative, not the normal default.

Completed/closed entries may remain in the list. The runner skips locally settled
work and does not rewrite order during closeout. Explicit planning cleanup may
remove old entries. An external issue closure does not establish verified checks
or prerequisite output; reopening execution is deliberate.

Source unavailability blocks new admission; an assigned task can finish from its
snapshot. Failed assigned work is preserved and stops automatic advancement.
Drained/budget-exhausted runs settle instead of polling indefinitely for new tasks.

## Agent responsibility

A planning/review assistant changes order only when instructed. A coding agent
implements its recorded assignment and does not independently claim another task,
remove completed entries, or switch to the planning branch. Outpost owns execution
and completion receipts. A shared file is not a multi-host distributed lock.
