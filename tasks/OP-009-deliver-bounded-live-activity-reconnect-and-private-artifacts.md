---
planning_id: OP-009
status: published
order: 9
depends_on: ["OP-007", "OP-008"]
source: bootstrap-handoff
github_issue: 9
---
# OP-009 — Deliver bounded live activity, reconnect, and private artifacts

**Dependencies:** #7 (OP-007), #8 (OP-008)  
**Specification:** §§18–20, 23, 28.  
**Acceptance owners:** AC-15, AC-32–35, foundations for AC-48.

**Canonical task:** https://github.com/felixlheureux/outpost/issues/9  
**Repository copy:** Published planning snapshot; update the issue for current requirements.

### Outcome

The browser can observe actual execution, reconnect after a network gap, and inspect retained output without unbounded memory use, lost drafts, or unsafe artifact paths.

### Implementation requirements

1. Persist bounded structured activity/events with cursors and private artifact references. Define normalized task/phase/message/check/delivery/source-health events; business-state receipts remain authoritative.
2. Serve an initial consistent projection/cursor, replay newer events, then follow SSE. Support Last-Event-ID and explicit reset when retention removes the required range. Restarted web service reconstructs state from storage and owned-process observations.
3. Bound subscriber queues and disconnect/reset slow clients. Backpressure must not block provider/check execution or accumulate unlimited process output.
4. Store full allowed output under private run/attempt artifact paths. Expose bounded previews, truthful truncation, explicit downloads, pagination/tail bounds, and size/disk diagnostics. Active/unresolved artifacts are never removed by retention actions.
5. Resolve downloads from allowlisted IDs/manifests; reject traversal, symlink escape, directories, and nonregular files. Enforce the same private identity boundary as API/SSE.
6. Escape logs and sanitize supported Markdown; provider output cannot inject active HTML/script. Keep private environment values/tokens out of ordinary errors.
7. Update only affected frontend resources. Preserve composer text/focus, selection, expanded message readers, and reading position. Following live updates is explicit; reading earlier output does not jump to the bottom on each event.
8. Expose connection freshness and last meaningful event separately from process liveness. A network gap must not show a false stopped or live state.

### Failure and edge cases

Exercise large command output, incomplete Unicode/lines, reconnect after cursor expiry, a slow phone connection, simultaneous viewers, and artifact disappearance after a valid settled retention action. A missing preview cannot silently pretend the complete output was displayed.

### Acceptance criteria

- [ ] Web restart and SSE reconnect return actual run state with correct cursor/reset behavior.
- [ ] Large output/slow subscribers remain bounded without blocking execution.
- [ ] Logs remain inert; forbidden artifact paths and denied identities are rejected.
- [ ] Reading older messages and typing instructions survive new activity.
- [ ] Last-seen/connection state does not claim a process stopped merely because SSE disconnected.

### Verification and handoff

Use bounded stress fixtures and focused replay/path tests plus one reconnect browser journey. Handoff documents event schemas, replay bounds, artifact API, frontend invalidation strategy, and notification retention policy.
