---
planning_id: OP-008
status: published
order: 8
depends_on: ["OP-007"]
source: bootstrap-handoff
github_issue: 8
---
# OP-008 — Integrate the real Codex App Server adapter

**Dependencies:** #7 (OP-007)  
**Specification:** §§11–14, 23, 28.  
**Acceptance owners:** AC-16, AC-26–31 at adapter level, real-provider portion of AC-38.

**Canonical task:** https://github.com/felixlheureux/outpost/issues/8  
**Repository copy:** Published planning snapshot; update the issue for current requirements.

### Outcome

An owned task can run through installed Codex using its actual supported App Server stdio protocol, with real capabilities/settings/session identity and structured results.

### Implementation requirements

1. Inspect the installed/supported Codex version and current official protocol schemas. Pin/document compatibility. Reuse useful behavior from `scripts/runner_codex.py` without copying Python transport assumptions blindly.
2. Implement a narrow adapter boundary for capabilities, session start/resume, turn start, steering, interruption, events, and structured completion. Keep provider-specific IDs/errors inside this adapter and durable receipts.
3. Discover supported model/reasoning/permission choices. Record actual selected model, reasoning, execution permissions, provider version, session ID, and turn ID. Unsupported capabilities remain explicit; no invented defaults or silent emulation.
4. Start the owned provider with bounded stdio handling, stderr capture, cancellation, frame/message limits, and well-defined process settlement. Do not buffer all output in memory.
5. Compose immutable effective prompts from runner contract, repository/project instructions, pinned issue snapshot, labeled workspace evidence, and authenticated operator direction. Preserve trust boundaries; source/log content is not operator control.
6. Parse structured handoff fields into proposed results, changed paths, claimed checks, blockers, and summary. Provider completion and claimed checks remain unverified until the runner's own policy settles.
7. Resume an explicit saved session when identity/capability/workspace agree. A missing or rejected session is a clear failure; fresh conversation requires explicit selection and a new linked attempt.
8. Normalize provider failures into actionable authentication, protocol, permission, interruption, and unknown-outcome states. Provider permission/OS boundaries cannot be elevated by prompt text.

### Failure and edge cases

Malformed/oversized frames, stderr spam, lost terminal acknowledgement, missing executable, authentication expiry, and unexpected process exit all preserve task assignment and receipts. Avoid automatically retrying a turn whose effects are unknown.

### Acceptance criteria

- [ ] Fixture protocol sessions prove start/events/result/error/cancellation behavior and resource bounds.
- [ ] Actual discovered settings are visible and stored with the attempt.
- [ ] Saved-session failure never silently starts a fresh conversation.
- [ ] A real authenticated Codex task performs a small disposable repository change with recorded identities.
- [ ] Agent claims are not marked runner-verified checks or delivery.

### Verification and handoff

Keep local protocol tests independent of paid-model credentials. Separately record the real version/account smoke result. Handoff supplies capabilities, normalized events, task handoff schema, steering/interrupt calls, and actual session-recovery rules for Work/controls.
