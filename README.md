# Outpost

Repository: https://github.com/felixlheureux/outpost

A private control room for your coding agents.

Outpost is intended to run on your Linux host and be reachable from your Mac or phone through Tailscale. GitHub Issues contain the work specifications; a small planning-branch manifest determines order; the host executes through an existing agent such as Codex and retains the results.

**Status:** Working Go/Echo + React starter: embedded SPA, TanStack Router/Query, SQLite migrations, local development shell, Git/gh adapters and checks. Repository cloning, private Tailscale identity, installer and coding worker remain implementation tasks. All 16 tasks are published as GitHub Issues.

## Installation

See [INSTALL.md](INSTALL.md) for working source-build commands and the planned guided operator installer. Normal setup will reuse GitHub/Codex/Tailscale sign-ins; no copied API keys or new Outpost account.

For local development, use Go 1.27.2, Node 24.19.0 and npm 11:

```sh
make setup
make dev
```

Open `http://127.0.0.1:5173`. `make build` produces `bin/outpost`, with the UI embedded. See [docs/development.md](docs/development.md) for running that executable, checks and generator setup.

## Start implementation

Read [docs/starter-handoff.md](docs/starter-handoff.md), `AGENTS.md`, the specification, and the assigned issue before coding. The next integration is [issue #2 — OP-002](https://github.com/felixlheureux/outpost/issues/2). The [live queue](https://github.com/felixlheureux/outpost/blob/outpost/queue/queue.json) is on the dedicated planning branch. Repository task files are reference snapshots; GitHub Issues contain current requirements.

## Start here

1. Read [AGENTS.md](AGENTS.md) for agent workflow and engineering constraints.
2. Read [Outpost-Technical-Specification.md](Outpost-Technical-Specification.md) for the complete contract.
3. Follow [Outpost-Implementation-Tasks.md](Outpost-Implementation-Tasks.md) or individual files under [tasks](tasks/), beginning with OP-001.
4. Open [Outpost-UX-Reference.html](Outpost-UX-Reference.html) locally in a browser to explore the intended layout and interactions.
5. Use [docs/project-setup.md](docs/project-setup.md) when setting up another installation or repository; this repository already has its tasks and planning branch initialized.

## The normal loop

Push code → review it with an assistant connected to GitHub → create/refine issues → deliberately queue work → let Outpost implement → inspect checks/delivery/history → review the pushed result again.

No direct public connection from ChatGPT to the private application is needed. Outpost reads GitHub outbound; Tailscale provides private operator access.

## Add and switch repositories in the intended app

With no projects, choose **Add repository**, paste a GitHub URL, and watch Outpost clone it onto the Linux host and configure its source. Successful setup opens that project's Queue. From the project menu, add another repository and switch between them. Checkouts are reused, and each project keeps its own queue/settings/history/drafts. While A runs, browsing B leaves A pinned in the global run bar; only one coding task executes at a time. Existing local checkout registration is an advanced option.

This flow is specified for implementation in issues #3–#6 and its execution behavior in #7; it is not already running code.

## Chosen stack

Go + Echo; React/TypeScript/Vite/Tailwind/shadcn; TanStack Router + Query; SQLite; native Git + GitHub CLI. SSE, user systemd and Tailscale integration follow in later tasks. Vite builds static assets embedded in Go. A production Vite/Node dashboard server is not required. Git, gh, Codex, Tailscale, systemd, and repository toolchains remain host prerequisites for the finished product.

## Starter contents

| Path | Purpose |
| --- | --- |
| `AGENTS.md` | Coding/planning instructions and validation cadence |
| `INSTALL.md` | Operator installation, dependencies/authentication, first run and lifecycle |
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
| `tasks/` | Sixteen published implementation-task reference snapshots |
| `issue-map.json` | Stable OP planning IDs mapped to actual GitHub Issues |
| `implementation-plan.json` | Machine-readable bootstrap order and dependency map |

## Queue authority

The **live** queue is `queue.json` at the root of the configured `outpost/queue` branch. The file under `planning-branch/` in this starter is only a seed and schema example. Do not read it as the current queue or maintain both copies as synchronized authorities.

JSON does not support comments. Explanations live beside the seed and in `docs/queue.md`; the runtime manifest stays minimal and valid JSON.

## Implementation status and release

Pinned dependencies and working source-build/check commands are committed. See [architecture](docs/architecture.md), [dependency decisions](docs/dependencies.md), and the [starter handoff](docs/starter-handoff.md). Subsequent tasks connect real host, source, provider, and UI behavior. There are no published release assets or working operator installer yet. License/publication policy remains the owner's choice; upstream component notices are preserved separately.
