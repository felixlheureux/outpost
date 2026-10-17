---
planning_id: OP-005
status: published
order: 5
depends_on: ["OP-004"]
source: bootstrap-handoff
github_issue: 5
---
# OP-005 — Implement the shared planning-branch queue

**Dependencies:** #4 (OP-004)  
**Specification:** §§8, 18–19, 26.1–6.  
**Acceptance owners:** AC-40–41, AC-44–45, AC-52.

**Canonical task:** https://github.com/felixlheureux/outpost/issues/5  
**Repository copy:** Published planning snapshot; update the issue for current requirements.

### Outcome

All planners and Outpost use one ordered issue-number list. Reordering changes one file and leaves issue bodies and the coding checkout alone.

### Implementation requirements

1. Read/write `queue.json` on configured `outpost/queue` through GitHub APIs. Validate `{"version":1,"issues":[...]}` with distinct positive issue numbers, bounded size/count, supported version, and valid references. Do not accept a partial queue from a malformed document.
2. Keep planning branch operations outside the execution checkout. No pull, branch switch, merge, or task-closeout queue rewrite is allowed. Missing branch/file gets a clear setup action using a known base ref; never reset an existing branch.
3. Cache manifest blob SHA, ordered identities, fetched time, and health. Resolve issue references with bounded reads and preserve source links. Do not treat private/inaccessible responses as proof an issue was deleted.
4. Expose read/sync/replace and append/remove/move operations through the canonical queue service. Require expected manifest SHA for writes. A stale SHA returns conflict with current order and preserved intended edit.
5. Reapply a move only after an explicit fresh read using current neighbors/order. Never retry the stale complete list over another planner's changes, maintain per-issue numeric ranks, or rewrite every issue.
6. Track operation IDs and reconcile uncertain writes against current blob/payload/commit observations. Keep success, rejection, and unknown separate.
7. Creation-plus-queueing is a two-step flow: if issue creation succeeds but append fails, expose the created issue in Inbox and retry append only. An issue existing in the list is a no-op append, not a duplicate entry.
8. Define source unavailability and manual Sync behavior. Background refresh is bounded/configurable; a worker always refreshes at admission independently of the UI poll.

### Failure and edge cases

Completed/closed entries may remain in the list and are skipped with provenance; removal is an explicit planning action. Removing an active issue does not interrupt its pinned assignment. Shared queue storage does not create distributed execution locking across separate installations.

### Acceptance criteria

- [ ] One reorder creates one manifest change and no issue edits or checkout changes.
- [ ] Two stale clients cannot silently overwrite each other; conflict preserves intended order.
- [ ] Invalid versions, duplicates, missing/inaccessible references, and source outages are explained.
- [ ] An uncertain write is inspected before a retry.
- [ ] Partial creation/append failure queues the same created issue on recovery.
- [ ] The current manifest SHA and source-health state are available to scheduler/UI.

### Verification and handoff

Use a focused fake GitHub contents service for SHA races/uncertainty and one real planning-branch read/write acceptance. Record checkout HEAD/status before and after to prove queue operations did not change it. Handoff documents manifest schema, error/receipt DTOs, and atomicity limits.
