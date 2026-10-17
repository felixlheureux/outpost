---
planning_id: OP-011
status: published
order: 11
depends_on: ["OP-008", "OP-009", "OP-010"]
source: bootstrap-handoff
github_issue: 11
---
# OP-011 — Own validation receipts and bounded repair

**Dependencies:** #8 (OP-008), #9 (OP-009), #10 (OP-010)  
**Specification:** §16 and §§9, 12, 18, 21.7, 29.  
**Acceptance owners:** AC-17–23.

**Canonical task:** https://github.com/felixlheureux/outpost/issues/11  
**Repository copy:** Published planning snapshot; update the issue for current requirements.

### Outcome

Outpost can prove which required checks ran against which inputs, reuse valid evidence, repair actual failures within a bound, and recover owned check results without repeating work unnecessarily.

### Implementation requirements

1. Resolve check plans from operator-configured profiles and accepted task criteria. Distinguish required/optional checks and runner-owned results from agent claims. Imported/issue text can select approved profiles, not invent executable commands.
2. Run argv-based owned check processes with defined working directory/environment policy, process identity, timeout/cancellation, bounded output, and durable terminal receipts. Keep workspace exclusion while any owned check can still write.
3. Define relevant input identity using tracked/relevant-untracked code, configuration, fixtures, tool/check contract, and declared supplemental inputs. Keep title, queue position, status, completion notes, and UI metadata out of a code check's identity unless a real check explicitly consumes them.
4. Format/generate before final acceptance. A no-op formatter or metadata-only closeout must not invalidate otherwise applicable receipts. Actual relevant code/config/criterion changes invalidate affected checks. Never reuse solely because a task ID matches.
5. Store command snapshot, generation, input/contract digest, start/end/duration, process identity, exit/signal, case-count requirements, status, and artifact links. A configured nonzero-case check cannot pass on zero tests.
6. On restart, reconcile the exact child or terminal receipt before rerun. Unknown process completion stays unknown. A surviving validation child blocks another writer and has an actionable stop/inspect path.
7. Send bounded real failure evidence back to the same assignment for repair. Default repair limit is three cycles; unchanged repeated failure/exhaustion stops with evidence. Missing credentials/external services remain explicit blockers rather than fake passes.
8. Integrate Checks UI: required/optional, passed/reused/failed/interrupted/not-run, duration, relevant identity, reason for reuse/invalidation, and retained output. Local success cannot settle until all required policy is met.

### Failure and edge cases

Cover supervisor death after check completion, child survival, output overflow, timeout/signal, zero collected tests, meaningful operator contract changes, and a no-op formatter. A terminal agent response does not settle a check process.

### Acceptance criteria

- [ ] Real check failure reaches bounded repair and cannot be reported as a pass by prose.
- [ ] A valid terminal receipt survives supervisor interruption and is reused after reconciliation.
- [ ] Metadata edits/no-op formatting do not trigger an unchanged full gate.
- [ ] Relevant code/config/contract changes reject stale evidence.
- [ ] Zero-case requirements, surviving children, and exhausted repair have clear outcomes.
- [ ] The UI explains what ran, what was reused, and why.

### Verification and handoff

Use small executable fixtures and disposable real Git inputs for receipt identity/recovery; avoid slow monorepo gates for fixture behavior. Handoff defines final acceptance contract, evidence generation, repair state, and the verified candidate identity used by Git delivery.
