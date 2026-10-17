---
planning_id: OP-013
status: published
order: 13
depends_on: ["OP-009", "OP-010", "OP-011", "OP-012"]
source: bootstrap-handoff
github_issue: 13
---
# OP-013 — Make history, blockers, and recovery actionable

**Dependencies:** #9 (OP-009), #10 (OP-010), #11 (OP-011), #12 (OP-012)  
**Specification:** §§8.6–7, 12, 15, 21.9–10, 22, 28.  
**Acceptance owners:** AC-14–18, AC-24, AC-31, AC-49.

**Canonical task:** https://github.com/felixlheureux/outpost/issues/13  
**Repository copy:** Published planning snapshot; update the issue for current requirements.

### Outcome

The owner can understand an earlier attempt, find its evidence, and recover current work through ordinary UI actions without editing SQLite, deleting locks, or reading raw process logs first.

### Implementation requirements

1. History lists real runs and linked attempts, including launch failures, blocked/interrupted outcomes, continuations, settings, check/delivery summary, and pending GitHub reporting. Preserve original outcomes.
2. Historical detail is read-only and links to current work/continuation explicitly. Never let an old attempt's composer or stop button issue controls against a newer target implicitly.
3. Use one failure/attention panel near the current composer: concrete reason, known surviving work/processes, last observation, and relevant action. Authentication, failed checks, invalid queue, baseline mismatch, missing session, and surviving child require different guidance.
4. Reuse the canonical continuation, stop, source-sync, and reporting operations. Recovery must not become a second unguarded launch path or a set of arbitrary shell commands.
5. Connection diagnostics show private address, actual Tailscale/systemd observations, allowed identity state, provider/source readiness, version, state/disk use, and last useful observation. Sanitize sensitive details; expose bounded diagnostic artifacts separately when needed.
6. Provide run/attempt/turn/message/check/delivery/artifact navigation without adding more primary app sections. Attempt details expose captured issue/manifest revisions and actual model/reasoning/permissions; missing imported fields say Not recorded.
7. Manual settled-artifact retention/archive is bounded and never deletes active/unresolved evidence. Explain what remains after an artifact is removed. No automatic retention framework is required.

### Failure and edge cases

Old history remains readable when the provider session no longer exists. An unavailable host observation is unknown, not proof of no process. Recovery of reporting is separate from recovery of coding. A historical success cannot be rewritten into a different outcome by a later attempt.

### Acceptance criteria

- [ ] Interrupted/blocked/failed/startup runs and their continuations are navigable with immutable evidence.
- [ ] Historical controls cannot accidentally target current work.
- [ ] Current blockers offer appropriate ordinary recovery and retain instruction drafts.
- [ ] Source/provider/systemd uncertainty remains visible and actionable.
- [ ] Retention actions reject active/unresolved artifacts and do not expose private files.

### Verification and handoff

Use one cross-state browser journey backed by stored fixture runs and focused destructive-retention guards. Handoff documents diagnostic meanings, blocker-action mapping, and recovery links for phone acceptance.
