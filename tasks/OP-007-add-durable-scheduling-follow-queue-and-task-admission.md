---
planning_id: OP-007
status: published
order: 7
depends_on: ["OP-002", "OP-003", "OP-005"]
source: bootstrap-handoff
github_issue: 7
---
# OP-007 — Add durable scheduling, Follow queue, and task admission

**Dependencies:** #2 (OP-002), #3 (OP-003), #5 (OP-005)  
**Specification:** §§9–12, 18–19, 26.  
**Acceptance owners:** AC-09–16, AC-25, AC-40–44, AC-51, AC-55–58.

**Canonical task:** https://github.com/felixlheureux/outpost/issues/7  
**Repository copy:** Published planning snapshot; update the issue for current requirements.

### Outcome

A run admits exactly one owned task at a time, follows fresh queue changes between tasks, and preserves the active assignment across planning edits and failures.

### Implementation requirements

1. Define run/attempt/execution-generation state, captured settings/budget/mode, immutable assignment snapshot, and the reservation/lock protocol shared with systemd. Use explicit expected-revision guards and unique constraints.
2. Default Follow queue to fresh manifest and relevant issue/dependency reads before each admission. Cache failures cannot authorize new work. The assigned task keeps its issue-body digest, manifest SHA/position, workspace baseline, instructions, and policy.
3. Walk manifest order after excluding settled generations, closures, holds, unavailable dependencies, and already-owned tasks. Do not sort by priority. Validate dependency graph and usable predecessor outputs, not just a done label.
4. Add optional Fixed scope as a captured ordered identity set; later additions stay out. Still refresh current issue state/definitions and prerequisites. Manual single-task and instruction runs remain explicit kinds and do not accidentally advance the queue.
5. Source requests happen outside DB transactions. After fetching, atomically compare local control/source observations and claim one assignment. Define the observation/assignment cutover honestly; remote writes after observation apply at the next boundary.
6. Check stop and budget at the same guarded admission boundary. New queue additions can join within the remaining budget; removals/reorder affect pending work only. Record the reason for skips and selected task.
7. Count successful tasks only after checks/delivery settle. Failure/blocked/interrupted work stops automatic advancement and keeps unfinished edits assigned. An empty eligible set distinguishes drained, budget reached, externally closed, and truly blocked work.
8. Reconcile surviving work before another assignment. A completed task remaining in the manifest is skipped even if GitHub reporting is pending. Source body changes do not implicitly reopen a local execution generation.

### Managed repository onboarding

Project browsing and repository setup are independent of the single global coding slot. A run remains scoped to its recorded project and never switches when the user selects another repository. Do not automatically start B after A drains. After an explicit stop/interruption of A, release the global slot only when all owned coding/check processes are settled and uncertain effects reconciled; unfinished A remains assigned to its own checkout/history. B may then start in its separate workspace without discarding or repairing A first. Resuming A later is explicit and rechecks its preserved assignment/baseline. If A still owns a child/process, explain why B cannot start.

### Failure and edge cases

External issue closure is not check evidence. A changed checkout may invalidate predecessor availability. Queue outage lets active work finish but blocks the next admission. A drained run settles rather than polling forever. A run reservation is not released just because a display status looks terminal.

### Acceptance criteria

- [ ] With `[72,41,88]` and `72→41`, the first eligible task is 41 and explanations remain visible.
- [ ] A newly queued task runs at the next eligible boundary within budget, while the active task stays pinned.
- [ ] Fixed scope excludes additions and both modes honor source outages/stop/baseline checks.
- [ ] Concurrent starts/control races produce one writer and no unwanted next task.
- [ ] Failed assigned work is preserved and not abandoned for another task.
- [ ] Completion/closure/reopen behavior preserves execution generations and provenance.

- [ ] Selecting/adding another project never retargets a running assignment or starts a second coding worker.
- [ ] After safe settlement of A, B can run while A's preserved unfinished checkout remains recoverable; surviving A processes still prevent a conflicting start.
- [ ] A drained run does not automatically jump to another project's queue.

### Verification and handoff

Use real SQLite concurrency, temporary Git baselines, and a deterministic fixture worker. Exercise source-change/admission timing and stop-after-task races. Handoff includes the assignment DTO, transition guards, next-eligible projection, and provider lifecycle integration point.
