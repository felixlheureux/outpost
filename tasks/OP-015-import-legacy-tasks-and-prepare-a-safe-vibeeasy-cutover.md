---
planning_id: OP-015
status: published
order: 15
depends_on: ["OP-004", "OP-005", "OP-007", "OP-012"]
source: bootstrap-handoff
github_issue: 15
---
# OP-015 — Import legacy tasks and prepare a safe VibeEasy cutover

**Dependencies:** #4 (OP-004), #5 (OP-005), #7 (OP-007), #12 (OP-012)  
**Specification:** §§9, 16, 27.  
**Acceptance owners:** AC-09, AC-11, AC-36, AC-45, AC-51–52.

**Canonical task:** https://github.com/felixlheureux/outpost/issues/15  
**Repository copy:** Published planning snapshot; update the issue for current requirements.

### Outcome

Existing Markdown/VibeEasy work can be previewed and mapped to GitHub Issues and the shared manifest without duplicate tasks, fabricated success, or competing runners.

### Implementation requirements

1. Inspect current VibeEasy task/queue/check conventions at implementation time. The previously reviewed `runner-dashboard/` and `scripts/runner_*.py` are behavioral references; do not assume every format stayed unchanged.
2. Implement a bounded general Markdown importer/exporter and focused VibeEasy adapter using a maintained metadata parser. Keep source identity, original number/path, requirements, dependencies, holds, and validation-profile references.
3. Preview create/update/skip actions, unresolved dependencies, duplicate/colliding IDs, unsupported policy, queue order, imported historical status, and baseline requirements before applying.
4. Apply with durable per-issue mapping/receipts, then a separate manifest update. A crash or append conflict resumes from created identities; no all-or-nothing GitHub/SQLite transaction is claimed.
5. Preserve imported historical outcomes as unverified provenance. Missing model/permission/check fields remain Not recorded. Imported done prose alone cannot establish a check receipt or predecessor code in the current checkout.
6. Export a readable snapshot including source issue identity/revision. Original task files and exports never become a second silently writable source authority.
7. Write a cutover runbook: test disposable checkout first; settle old worker/check children; inspect/preserve the chosen checkout; confirm issue/dependency/hold mappings; prevent two writers; run one small new-path task; retain the old installation for fallback until accepted.
8. If VibeEasy still requires updating original evidence files, implement only a narrow configured closeout action with defined inputs/checks. It must not restore competing queue ownership or require redundant code gates after metadata-only updates.

### Failure and edge cases

Importing again does not duplicate issues. Conflicting source IDs do not overwrite unrelated work. The old Python journal is never treated as an Outpost Goose database; history import reads a preserved/consistent copy.

### Acceptance criteria

- [ ] A representative current VibeEasy task set previews with accurate mappings/warnings.
- [ ] Apply/retry preserves created issue identities and valid dependency/order references.
- [ ] Imported completion remains distinct from runner verification and baseline evidence.
- [ ] Queue writes target the planning branch and never silently rewrite old runner sources.
- [ ] The runbook prevents concurrent ownership and preserves old source/history.

### Verification and handoff

Use a small representative fixture plus a read-only preview of actual current tasks. A real cutover is separate owner/environment acceptance, not implicit permission to alter the old installation. Handoff includes mapping/report, unsupported cases, and rollback boundaries.
