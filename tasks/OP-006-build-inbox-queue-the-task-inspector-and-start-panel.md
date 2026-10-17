---
planning_id: OP-006
status: published
order: 6
depends_on: ["OP-003", "OP-004", "OP-005"]
source: bootstrap-handoff
github_issue: 6
---
# OP-006 — Build Inbox, Queue, the task inspector, and start panel

**Dependencies:** #3 (OP-003), #4 (OP-004), #5 (OP-005)  
**Specification:** §21.1–6 and §§8, 10, 19.  
**Acceptance owners:** AC-07–08, AC-39–41, AC-45, AC-47.

**Canonical task:** https://github.com/felixlheureux/outpost/issues/6  
**Repository copy:** Published planning snapshot; update the issue for current requirements.

### Outcome

The owner can capture an issue, refine it, deliberately queue it, reorder pending work, and understand what would run next without leaving a compact workspace.

### Implementation requirements

1. Follow the prototype's density, restrained controls, three-view navigation, and selected task inspector. Use actual source data and honest loading/empty/error states.
2. Implement Queued/Inbox tabs. Inbox means open unqueued issues, not a GitHub draft feature. Keep task selection and list position through refresh, edits, tab changes, and opening/closing details.
3. Show position, issue number/title, readiness, dependency/hold reason, and source link. The next eligible task may differ from row one; expose both clearly. Do not add a priority sort that competes with manifest order.
4. The inspector provides outcome/criteria, dependencies, effective checks/delivery, historical link, Edit, Queue/remove/hold, and accessible Move earlier/later actions. Do not require drag-and-drop.
5. Use one compact new/edit form. Title/outcome/criteria are primary; metadata/policy overrides are optional details. Save defaults to Inbox; Add to the end of the queue is explicit. Active specification edits clearly apply to a future attempt and link to Work for live direction.
6. Show saving/saved/conflict/unavailable and partial issue-created-but-not-queued states. Retain form text and intended order on failure. Disable/reconcile conflicting controls during unresolved operations without losing the user's input.
7. Implement a start panel showing project/branch, next eligible task/blockers, Follow queue default, optional Fixed scope, model/reasoning/actual permissions, delivery, task limit, optional runtime budget, and instructions. Until execution integration exists, clearly identify the missing capability rather than simulate success.
8. Source details and settings live behind secondary actions. Last successful sync and actionable failures are visible without exposing host credentials or making ordinary users manage JSON.

### Failure and edge cases

Selection survives a source reorder; removal/closure of the selected issue has an explicit state. Two-tab conflicts keep both source and local intent inspectable. A failed append must not invite another Create issue click as the only recovery path.

### Acceptance criteria

- [ ] Create → Inbox → inspect/edit → queue → move works against the backend contracts.
- [ ] Inspector navigation preserves list context and unsaved form text.
- [ ] Queue position and eligibility are distinguishable, including dependencies and holds.
- [ ] Stale SHA/source edit conflicts preserve drafts and never claim a rejected write was saved.
- [ ] The start panel makes its scope and effective settings clear without a second mandatory approval screen.
- [ ] Keyboard and basic narrow-screen operations work before the later responsive pass.

### Verification and handoff

One browser journey covers creation, partial failure, reorder conflict, and task selection; avoid component snapshots mirroring markup. Handoff identifies the real start request payload and the selected issue/source revision used by the scheduler.
