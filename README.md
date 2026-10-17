# Outpost

Repository: https://github.com/felixlheureux/outpost

A private control room for your coding agents.

Outpost is intended to run on your Linux host and be reachable from your Mac or phone through Tailscale. GitHub Issues contain the work specifications; a small planning-branch manifest determines order; the host executes through an existing agent such as Codex and retains the results.

**Status:** Specification, detailed backlog, interactive UX reference, and starter repository. The actual Go/React application is not implemented in this package.

## Start here

1. Read [AGENTS.md](AGENTS.md) for agent workflow and engineering constraints.
2. Read [Outpost-Technical-Specification.md](Outpost-Technical-Specification.md) for the complete contract.
3. Follow [Outpost-Implementation-Tasks.md](Outpost-Implementation-Tasks.md) or individual files under [tasks](tasks/), beginning with OP-001.
4. Open [Outpost-UX-Reference.html](Outpost-UX-Reference.html) locally in a browser to explore the intended layout and interactions.
5. Follow [docs/project-setup.md](docs/project-setup.md) after choosing the destination GitHub repository.

## The normal loop

Push code → review it with an assistant connected to GitHub → create/refine issues → deliberately queue work → let Outpost implement → inspect checks/delivery/history → review the pushed result again.

No direct public connection from ChatGPT to the private application is needed. Outpost reads GitHub outbound; Tailscale provides private operator access.

## Chosen stack

Go + Echo; React/TypeScript/Vite/Tailwind/shadcn; SQLite; SSE; user systemd; Tailscale. Vite builds static assets that are embedded in Go. A production Vite/Node dashboard server is not required. Git, Codex, Tailscale, systemd, and repository toolchains remain host prerequisites.

## Starter contents

| Path | Purpose |
| --- | --- |
| `AGENTS.md` | Coding/planning instructions and validation cadence |
| `.github/ISSUE_TEMPLATE/task.yml` | Structured ordinary GitHub task issue |
| `.github/ISSUE_TEMPLATE/bug.yml` | Reproduction-oriented bug issue |
| `.github/ISSUE_TEMPLATE/config.yml` | Keep ordinary issue creation available |
| `.github/pull_request_template.md` | Concise change/evidence/limitations handoff |
| `planning-branch/queue.json` | Empty branch seed; not the live main-branch queue |
| `scripts/init_planning.py` | Explicit, non-overwriting planning-branch initialization |
| `docs/queue.md` | Exact manifest contract, examples, conflicts, and agent responsibilities |
| `docs/task-authoring.md` | Issue structure, dependencies, and scope guidance |
| `docs/project-setup.md` | Initial repository setup and task publication |
| `docs/development.md` | Implementation/build/verification rules and completion handoff |
| `docs/prototype-reference.md` | How to interpret and use the interactive reference |
| `tasks/` | Sixteen detailed bootstrap implementation tasks |
| `implementation-plan.json` | Machine-readable bootstrap order and dependency map |

## Queue authority

The **live** queue is `queue.json` at the root of the configured `outpost/queue` branch. The file under `planning-branch/` in this starter is only a seed and schema example. Do not read it as the current queue or maintain both copies as synchronized authorities.

JSON does not support comments. Explanations live beside the seed and in `docs/queue.md`; the runtime manifest stays minimal and valid JSON.

## Implementation status and release

OP-001 establishes actual pinned dependencies and working build commands. Subsequent tasks connect real host, source, provider, and UI behavior. Do not add a nonfunctional CI workflow or claim release commands exist before they are implemented. License/publication policy remains the owner's choice; no license grant is invented here.
