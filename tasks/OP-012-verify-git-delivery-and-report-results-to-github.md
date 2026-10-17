---
planning_id: OP-012
status: published
order: 12
depends_on: ["OP-004", "OP-005", "OP-011"]
source: bootstrap-handoff
github_issue: 12
---
# OP-012 — Verify Git delivery and report results to GitHub

**Dependencies:** #4 (OP-004), #5 (OP-005), #11 (OP-011)  
**Specification:** §§16.9–17, 18, 26.7–9.  
**Acceptance owners:** AC-21, AC-24, AC-46, AC-51–52.

**Canonical task:** https://github.com/felixlheureux/outpost/issues/12  
**Repository copy:** Published planning snapshot; update the issue for current requirements.

### Outcome

Completion means the configured local/commit/push policy actually settled. A later GitHub reporting outage cannot cause repeated coding work or damage the shared queue.

### Implementation requirements

1. Support project-selected local_changes, commit, and push outcomes. Optional review is a policy, not a universal new approval flow. Record actual candidate, relevant check receipts, changed-path ownership, branch/upstream, and delivery observations.
2. Never bulk-stage unrelated files or reset/stash source to simplify delivery. Detect ambiguous pre-existing changes/commits and provide explicit recovery/investigation rather than guessing ownership.
3. Commit only the accepted candidate and intended owned files. Verify resulting commit and intended remote/branch before considering push settled. A post-check meaningful code change requires relevant revalidation; allowed metadata closeout does not repeat an unchanged gate.
4. Reconcile uncertain commit/push outcomes through Git facts and remote containment. Do not create another commit or blindly repeat effects merely because an acknowledgement was lost.
5. Settle local task/attempt/completed count only when required policy is met. Preserve immutable receipts and separate optional review outcome from provider text.
6. Persist a narrow GitHub reporting operation with stable identity/marker after local completion. Post issue comment with outcome, delivered commit/branch, check summary, limitations, and retained local run identity. Exclude tokens, host paths, full prompts, and large private logs.
7. Close the issue according to configured completion policy only. Reporting/closing failure leaves local completion settled and visibly pending; retry/reconcile that remote step without another coding attempt. Unknown comment creation checks the stable marker before replay.
8. Never remove/reorder queue entries as part of closeout. The scheduler skips settled generations. External closure does not fabricate local evidence; reopening execution is explicit.

### Failure and edge cases

Handle detached/no-upstream configuration, remote rejection, authentication loss, commit created before crash, push succeeded before status write, report succeeded before acknowledgement, and issue already closed. Preserve uncertainty without deadlocking unrelated settled reporting forever.

### Acceptance criteria

- [ ] Each delivery mode has a real receipt proving its configured outcome.
- [ ] Unrelated changes remain untouched and ambiguous ownership blocks safely.
- [ ] Commit/push recovery inspects existing effects rather than duplicating them.
- [ ] GitHub reporting outages remain pending while local success stays settled.
- [ ] Reporting resumes with the same operation identity and no new coding attempt.
- [ ] Completion does not rewrite the manifest; next work uses the existing order.

### Verification and handoff

Use local bare remotes for Git behavior and mocked GitHub outcomes; record a real configured push/comment/close journey separately. Handoff exposes outcome/reporting projections for History and the dependency-baseline receipt for the next task.
