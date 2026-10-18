# Runner-owned validation

OP-011 captures an operator check profile in each run policy. Configure a project
profile after repository setup:

```sh
outpost checks set --project PROJECT_ID --file /absolute/path/checks.json
outpost run --project PROJECT_ID --model MODEL --reasoning EFFORT
```

`run --check-profile NAME` selects another configured project profile.
`run --check-profile-file PATH` captures a one-run operator profile. A GitHub
issue may select the captured profile by name; `inherit` uses that captured
profile. Issue text, repository files, provider claims, and output cannot add
executable checks. Follow-queue runs currently capture one profile; an issue
selecting another configured name waits for a run with that profile.

A profile is JSON with a stable ID and argv-based local checks. `prepare` runs
formatters or generators before identity and final checks. Keep write-producing
commands in `prepare`; final checks must leave relevant inputs unchanged.

```json
{
  "id": "standard",
  "repair_limit": 3,
  "prepare": [
    {"id": "format", "name": "Format", "argv": ["make", "format"], "cwd": ".", "required": true, "execution_class": "local"}
  ],
  "checks": [
    {"id": "test", "name": "Tests", "argv": ["make", "check"], "cwd": ".", "required": true, "execution_class": "local"}
  ]
}
```

Each required check stores its command, selected nonsecret environment values,
contract digest, content identity, process PID/start ticks, terminal result,
case count when configured, and a private bounded output file. Set
`require_cases: true` with a `case_count_pattern` containing one capture group
when zero selected tests must fail. `supplemental_inputs` lists ignored fixtures
or generated inputs that affect the check. The default content identity covers
tracked and relevant untracked files, including their working-tree bytes; Git
metadata, issue title/status/order, and Outpost state do not change it.

A check child inherits the workspace lock and writes an atomic terminal file.
A surviving child blocks another writer. `outpost checks recover --run-id RUN_ID`
reconciles terminal evidence after the recorded worker has stopped; it refuses
an active worker or lock. Missing terminal evidence remains unknown and does
not rerun automatically. Use `outpost stop --run-id RUN_ID` to stop an active
recorded worker; the Checks panel shows the check PID and retained output.

On a real required failure, the worker sends a bounded output excerpt to the
same pinned assignment and starts a linked provider repair attempt. Three
cycles are allowed by default. An unchanged failure or exhausted limit holds
the assignment with its receipts. Successful local checks move the attempt to
closing with a candidate identity and receipt. OP-012 owns Git delivery and
final settlement; a provider handoff or local pass alone cannot mark success.
