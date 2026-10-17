---
planning_id: OP-014
status: published
order: 14
depends_on: ["OP-006", "OP-010", "OP-013"]
source: bootstrap-handoff
github_issue: 14
---
# OP-014 — Finish phone/tablet operations and accessibility

**Dependencies:** #6 (OP-006), #10 (OP-010), #13 (OP-013)  
**Specification:** §21, especially 21.11–12; §22.  
**Acceptance owners:** AC-32, AC-47–50.

**Canonical task:** https://github.com/felixlheureux/outpost/issues/14  
**Repository copy:** Published planning snapshot; update the issue for current requirements.

### Outcome

The same app is comfortable for quick Tailscale check-ins from a phone and does not squeeze several desktop columns into a tablet viewport.

### Implementation requirements

1. Inspect real routes at 320, 390, 768, 820, 1024, and 1440 CSS pixels. Collapse the sidebar and task/context panels before content becomes cramped. Use a full-width inspector sheet or detail route at narrow sizes, preserving the queue's selection/scroll on return.
2. Keep the current run state and finish/stop/recovery controls reachable. Overflow actions must work on touch without hover. The composer remains full-width and usable with the mobile keyboard open.
3. Verify core phone flows: current status, inspect blocker, send direction, finish-after-task/undo, interrupt, saved/fresh continuation, inspect checks, and move next work.
4. Prevent application-level horizontal overflow. Wrap issue titles, source errors, branch/commit identifiers, metadata, and form controls. Restrict horizontal scrolling to actual code/diffs/logs where it helps.
5. Use comfortable touch targets, 16px editable text where necessary, logical focus order, visible focus, native labeled controls, accessible tab/dialog behavior, Escape/close return focus, and keyboard alternatives for queue movement.
6. Check light/dark contrast and semantic statuses paired with text. Honor reduced-motion and avoid continual animation/fake progress.
7. Reconnect after sleep/network loss without discarding drafts, falsely claiming liveness, jumping the reader, duplicating a message, or reissuing an execution control automatically.
8. Remove redundant settings/navigation and prototype-only explanatory text from product flows. Keep implementation details secondary unless they explain a meaningful decision.

### Failure and edge cases

Use a long task title, long unbroken branch/path, empty Inbox, source conflict, stop-pending state, and unknown message receipt. Test a dialog opened after scrolling and a draft typed before a route change.

### Acceptance criteria

- [ ] Every target width has readable primary content without page overflow or squeezed fixed columns.
- [ ] Phone operational flows are reachable with the keyboard open and touch controls usable.
- [ ] Keyboard users can inspect/edit/move tasks, send instructions, and close dialogs with sensible focus return.
- [ ] Reconnection/navigation preserves scoped drafts and reading position.
- [ ] Light/dark/status/focus contrast passes the chosen accessibility checks.

### Verification and handoff

Use representative browser viewport checks and visual inspection, not a separate snapshot for every element/state. Record actual device verification separately from emulation. Handoff includes viewport evidence, any known browser limitations, and changes from the concept made for better real usability.
