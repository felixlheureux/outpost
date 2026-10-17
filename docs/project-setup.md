# Create the repository and planning source

This repository is already initialized at https://github.com/felixlheureux/outpost. Its implementation issues are published and mapped in `issue-map.json`; its live manifest is on `outpost/queue`. The steps below explain initialization for another repository or recovery of a missing setup component.

## Initial repository

Choose the destination repository and its visibility. This package does not
create a repository or claim one exists. Copy its contents into a fresh checkout,
or inspect and merge deliberately into an existing repository without overwriting
its instructions, tasks, or code blindly.

Review `AGENTS.md`, the specification, and the backlog. Commit the starter to the
intended default branch. Issue forms become available from GitHub after their files
are present on the default branch. An initial commit must exist before a planning
branch can be created.

No Go module, lockfile, executable, or CI gate is faked here. OP-001 establishes
actual supported dependency versions and working development/release commands.
Select a license/publication policy deliberately before representing reuse rights.

## Initialize the separate queue

Development setup prerequisites: Python 3 and an authenticated GitHub CLI with
permission to read the repository, create a branch, and create a contents file.
These helper prerequisites do not become dashboard runtime dependencies.

```sh
python3 scripts/init_planning.py --repo OWNER/REPOSITORY
```

Optional `--branch` and `--path` must match later project configuration. The helper
reads the default branch, creates the planning branch only if missing, and seeds
the queue only if missing. It never resets an existing branch, overwrites an
existing queue, changes the local checkout, or publishes implementation issues.

If an effect fails or its outcome is uncertain, inspect the returned GitHub error
and current branch/file. Re-running first reads existing state and preserves it;
there is no force flag.

## Publish the bootstrap tasks

1. Use individual `tasks/OP-xxx-*.md` bodies in dependency-ready order.
2. Search for an existing issue with the same OP ID before creating one; retain
   the actual issue identity in your publication mapping.
3. Keep OP IDs in titles for traceability, but translate dependencies to actual
   issue numbers in the Outpost settings block.
4. After creation, write the ordered real numbers to the planning manifest using
   its current SHA. A partial publication resumes from existing mappings.
5. Do not put `implementation-plan.json` into the live queue: it is a bootstrap
   task/dependency index with a different purpose and format.

Publication targets this repository only. Its 16 issues are already published;
reuse the mapping rather than creating another copy. No issues were created in
VibeEasy or another unrelated repository as a substitute.

## Implement and install

Start OP-001, reuse demonstrated existing behavior, and proceed through ready
dependencies. OP-002 establishes the real private host path. Complete real owner
acceptance before replacing the old runner. Preserve the old installation and
unfinished work until the cutover is accepted.
