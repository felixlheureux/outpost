# Author a task another agent can implement

Use the Coding task issue form or an ordinary issue with a clear title/body.
Creating the issue captures work in Inbox. Queue it explicitly when it should
become executable; do not use title prefixes or numeric priority fields as order.

## Useful body

- **Outcome:** concrete resulting behavior and the problem it resolves.
- **Acceptance criteria:** observable results and important failure/recovery cases.
- **Implementation notes:** relevant files/commits, constraints, boundaries, and
  focused verification context. State non-goals when they prevent scope creep.
- **Outpost settings:** optional structured references to configured policy.

```yaml
outpost_task: 1
kind: implementation
depends_on: []
hold_reason: null
check_profile: inherit
delivery: inherit
review: inherit
```

Put that YAML in one fenced block below `## Outpost settings`. Dependencies are
real issue numbers in the same configured repository. Use an existing configured
check profile or inherit the project default. Never place executable shell-check
definitions, secrets, or permission elevation in issue metadata.

Hold reason is a human explanation or null. A hold constrains eligibility; it does
not reorder the list. A canceled/externally closed dependency is not automatically
a verified success. Explain prerequisite baseline requirements when importing work.

Use a bounded task whose completion can be established. Do not split tiny coupled
edits into separate issues just to increase task count, or hide a whole platform
rewrite behind one vague acceptance bullet.

## During execution

The worker captures the source revision when assigning the task. Pending edits
apply at future admission. Editing the active issue does not steer the live agent;
use Work's instruction composer for current direction. A substantive live contract
change may invalidate affected check receipts. Prior snapshots remain history.

## Completion and review

Required runner checks and configured delivery establish completion. A provider's
claim, issue checkbox, closed state, or comment alone is insufficient. Reporting
to GitHub may remain pending after verified local success without repeating coding.

After reviewing pushed code, create follow-up tasks with evidence and explicit
scope. Unpushed files are not visible to an assistant reviewing only GitHub.
