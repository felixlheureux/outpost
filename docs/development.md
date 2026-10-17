# Development and delivery

## First implementation

OP-001 owns toolchain/version selection and the actual development commands.
Commit the chosen frontend package lockfile and Go module sums. Keep frontend
build generation before Go embedding; do not require Vite/Node in production.

Prefer direct Go services, explicit DTOs, sqlc queries and short transactions.
Package boundaries in the spec are responsibility suggestions, not an instruction
to create dozens of empty packages. Keep provider/GitHub/systemd effects narrow.

## Validation cadence

During a task, run focused tests for the behavior at risk. Format/generate before
the final required gate. Broaden validation when a changed contract, failure, or
unresolved concern warrants it. Reuse evidence only while its relevant inputs and
contract still apply. Do not rerun a full gate after a no-op formatter or a
documentation/status/queue-only closeout edit.

Meaningful risk areas include ownership races, unknown effects, immutable receipts,
source conflicts, input preservation, path confinement, and recovery. Avoid tests
that merely restate component markup or reversible configuration lines.

Once implementation exists, document the working commands here. Expected gate
categories are Go formatting/vet/tests, frontend typecheck/build, relevant browser
journeys, generated query freshness, and an embedded-executable smoke check.
Host/provider/account/device checks are separate evidence, never inferred from
fixtures. Do not publish shell commands for nonexistent targets as though they work.

## Handoff

Each task records the resulting behavior, relevant contracts/files, checks with
results, remaining limitations, and delivered identity when applicable. Preserve
the assigned issue/source revision and linked attempts. Checks and Git outcomes
are established by receipts and observation, not an agent's confident summary.

## Release boundaries

The first release is sequential and private. No hosted billing, multi-tenancy,
distributed execution, mandatory workflow designer, extra provider, or parallel
worktrees are required. A real issue-to-result loop used from Mac/phone matters
more than adding another settings screen or abstraction.
