---
planning_id: OP-010
status: published
order: 10
depends_on: ["OP-006", "OP-008", "OP-009"]
source: bootstrap-handoff
github_issue: 10
---
# OP-010 — Implement Work, live instructions, stop, and continuation

**Dependencies:** #6 (OP-006), #8 (OP-008), #9 (OP-009)  
**Specification:** §§14–15, 19, 21.2/7–9.  
**Acceptance owners:** AC-16, AC-25–31, AC-48–49.

**Canonical task:** https://github.com/felixlheureux/outpost/issues/10  
**Repository copy:** Published planning snapshot; update the issue for current requirements.

### Outcome

The owner can monitor and steer real work, request a clean stop after the task, interrupt immediately, and continue preserved work without losing typed instructions or targeting the wrong turn.

### Implementation requirements

1. Build Work around the prototype: task/settings/phase header, Activity/Changes/Checks tabs, optional wide context panel, and one stable composer. Show actual model/reasoning/permissions and completed count/budget. No invented percentage.
2. Activity uses bounded normalized events; Changes reads actual changed paths and lazy diffs; Checks integrates receipts as OP-011 lands. Keep task brief available when the context panel collapses.
3. Persist live message intent with client ID, payload digest, attempt/session/expected-turn target, and queued/sending/accepted/rejected/unknown states. Accepted means provider acknowledgement, not implementation success.
4. Reject stale targets without silently redirecting to another turn. Between managed turns, allow explicit next-turn delivery. Lost acknowledgement becomes unknown; do not automatically resend. Duplicate key/payload returns its receipt; changed payload conflicts.
5. Preserve the same composer through routes, events, rejection, interruption, and reload using scoped session storage. Retain text until delivery is acknowledged/reconciled, and keep unknown submissions inspectable. Never carry a draft into another project's active task automatically.
6. Implement durable Finish task & stop, Undo stop before next admission, and Interrupt now. Keep stop state globally visible; interruption observes provider and owned-child settlement and does not merely set a local UI flag.
7. After interruption/blocking, offer Continue with instructions, saved-session resume when available, and explicit fresh conversation. Create linked attempt history and retain budget/count. State whether queue processing resumes afterward.
8. Active specification edits remain future task revisions; live steering records current operator instructions and can change the validation contract. Keep old source/prompt evidence immutable.

### Failure and edge cases

Cover message dispatch racing turn completion, stop racing next assignment, continuation while an owned child survives, missing saved session, repeated button presses, and browser reconnection after an unknown submission. Avoid two composers or controls on an old historical attempt.

### Acceptance criteria

- [ ] Actual turn-directed instructions have visible durable delivery state.
- [ ] Stale/unknown/duplicate messages behave truthfully and preserve user input.
- [ ] Finish-after-task prevents the next guarded admission; Undo cancels only while applicable.
- [ ] Interrupt preserves diff/session/assignment and waits for real settlement.
- [ ] Saved/fresh continuation is explicit and retains remaining budget/history.
- [ ] Queue/Work/History navigation and SSE never discard unsent instructions or move a reader unexpectedly.

### Verification and handoff

Use targeted protocol/control race tests and a browser journey typing before navigation, stale-target rejection, interruption, and continuation. Add a real provider steering/interrupt acceptance separately. Handoff includes message/stop/continuation receipts and the contract-change signal for validation.
