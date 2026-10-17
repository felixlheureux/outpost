---
planning_id: OP-003
status: published
order: 3
depends_on: ["OP-001", "OP-002"]
source: bootstrap-handoff
github_issue: 3
---
# OP-003 — Register projects and build the compact application shell

**Dependencies:** #1 (OP-001), #2 (OP-002)  
**Specification:** §§8–9, 19, 21–22, 25.  
**Acceptance owners:** AC-11, AC-13, initial AC-47/50.

**Canonical task:** https://github.com/felixlheureux/outpost/issues/3  
**Repository copy:** Published planning snapshot; update the issue for current requirements.

### Outcome

The owner can register existing repository workspaces, inspect readiness, switch projects, and navigate Queue, Work, and History without losing the current run.

### Implementation requirements

1. Store project identity, canonical workspace/Git common directory, execution defaults, configured check profiles, Git delivery target, and source configuration references. Do not put credentials in public DTOs.
2. Inspect existing checkout/branch/remote, staged/unstaged/untracked paths, and provider prerequisites. Registration does not clone, reset, stash, switch branches, rewrite remotes, or authenticate the agent silently.
3. Detect filesystem aliases and duplicate checkout identities. A second project record cannot bypass workspace exclusion by referring to the same location differently.
4. Provide readiness with machine codes and concrete next actions for unavailable provider, dirty/unassigned work, changed baseline, missing profiles, missing source setup, and owned worker/check. Mark source-dependent readiness incomplete until OP-004/005 provides it.
5. Build the prototype's restrained shell: narrow desktop navigation for Queue/Work/History, project selection only when multiple projects exist, secondary project settings and connection diagnostics, and a persistent run bar that names its project/task.
6. Default landing is Work during active/input-required work, Queue when idle; honor deep links and deliberate navigation. Switching projects does not retarget an active run's controls or instruction draft.
7. Show actual empty/loading/unavailable states. The global run bar must use real server observations, distinguish stale connection from stopped execution, and link to the owning run even while another project's queue is open.

### Failure and edge cases

Unknown paths and non-Git directories produce readable errors. Archiving a project with unresolved work is rejected. Unsupported provider setup is a readiness reason, not permission to invent a model or claim the installation is ready.

### Acceptance criteria

- [ ] Two independent repositories can be registered and selected with separate settings.
- [ ] Alias registration cannot create competing ownership of the same checkout.
- [ ] Registration leaves Git state unchanged.
- [ ] The run bar remains correct through project/route changes.
- [ ] Readiness explains the failed prerequisite and an appropriate next action.
- [ ] Empty and populated shell layouts remain usable on desktop and a narrow phone.

### Verification and handoff

Use temporary real Git repositories for identity/baseline observations and one shell navigation journey. Handoff specifies project/readiness DTOs, settings revisions, source-configuration slots, and current-run navigation used by subsequent tasks.
