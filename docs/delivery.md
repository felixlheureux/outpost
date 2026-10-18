# Delivery and GitHub reporting

OP-012 connects the runner's passed check candidate to a selected delivery
policy. The private Start panel and `outpost run` capture the default policy:
`local_changes`, `commit`, or `push`. `local_changes` is the default. A pinned
issue may override the policy in its Outpost settings. Push requires an
attached branch with an upstream when the assignment is admitted. The branch,
upstream, and a digest of the remote URL are pinned before the provider starts.

At delivery, Outpost checks the candidate digest against the passed check
receipt, the branch, the clean admission baseline, and the exact changed path
set. Commit mode records intent before staging the listed paths and creates
one commit with an assignment marker. It checks the parent, changed paths,
and SHA. Push mode fetches the pinned branch to verify that it contains the
commit. A rejected or uncertain push remains pending. Outpost never force
pushes, resets, stashes, or edits the planning manifest.

The assignment settles only after the check and delivery receipts agree on
the candidate. A required review pauses after checks and before Git delivery.
The review decision is stored separately. Rejection with feedback starts a
linked provider repair attempt against the same pinned assignment, followed
by new checks. No commit or push occurs before approval.

After local success, a separate GitHub operation posts a bounded result
comment with a stable marker. `--close-issue` (or the Start panel setting)
closes the source issue after the comment is confirmed. GitHub failure leaves
local success settled. Retry without rerunning coding work:

```sh
outpost reports retry --run-id RUN_ID
```

If a worker stops after passed checks, inspect the worker and any check
child, then reconcile the exact recorded delivery:

```sh
outpost delivery recover --run-id RUN_ID
```

If delivery settled but unit/lock observation was temporarily unavailable,
retry only the terminal ownership check with `outpost delivery release
--run-id RUN_ID`. It cannot release a live worker or an active schedule.

The command requires an inactive worker and the workspace lock. It inspects
existing commit and remote facts before replaying an uncertain effect. A
candidate change after checks needs new validation; delivery refuses it. The
normal worker exits after a terminal schedule. The web service releases its
reservation only after observing an inactive systemd unit and an obtainable
workspace lock. Fixture results do not prove a real host push, user service,
Tailscale, or GitHub write permission.
