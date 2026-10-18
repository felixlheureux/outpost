<img src="docs/assets/outpost-logo.svg" alt="Outpost logo" width="88" height="88">

# Outpost

**A private control room for coding agents on your own Linux host.**

Outpost is a self-hosted dashboard for turning GitHub Issues into supervised Codex work. Plan tasks in a shared GitHub queue, start a run from your browser, follow its activity, and keep its checks, Git delivery, and result receipts together. The dashboard is designed for access from a laptop or phone over Tailscale.

Outpost runs as one Go executable with an embedded React interface and SQLite state. It reuses your host's GitHub CLI and Codex sign-ins; there is no Outpost account or separate production Node server.

## What it does

- **Plan in GitHub.** Issues hold task requirements. A single `queue.json` on the `outpost/queue` branch holds queue order. Outpost shows Inbox, Queue, eligibility, dependencies, and holds without creating another task authority.
- **Run on your host.** Managed repository checkouts and one global execution slot keep each assignment tied to its project. Follow queue refreshes the source before a new assignment and pins requirements once work starts.
- **Stay in control.** The Work view retains activity and artifacts, with scoped live direction, stop, and continue controls. Switching projects does not redirect an active run.
- **Verify the result.** Configured checks run against the candidate; selected local, commit, or push delivery has recoverable receipts. GitHub result reporting is tracked separately so a reporting retry does not rerun coding work.

## Try it from source

Requires Go 1.27.2, Node 24.19.0, npm 11, Git, and Make. The local preview needs no account login.

```sh
git clone https://github.com/felixlheureux/outpost.git
cd outpost
make setup
make build
./bin/outpost migrate
./bin/outpost serve --dev
```

Open [http://127.0.0.1:8080](http://127.0.0.1:8080) on the same machine. This is a loopback development preview. For frontend hot reload, run `make dev` and open `http://127.0.0.1:5173`. See [INSTALL.md](INSTALL.md) for private host setup and [development notes](docs/development.md) for checks and generator commands.

## How work moves

1. Add a GitHub repository URL. Outpost creates a managed checkout and connects its Issues and planning queue.
2. Refine and queue an issue. The next eligible task is chosen from the manifest, with dependency and hold rules applied.
3. Start a run with a Codex model, checks, and delivery policy. Follow its activity or send a scoped instruction from Work.
4. Inspect the check and delivery receipts. Outpost records the result locally and reports it to GitHub with a separate retry path.

Project switching changes what you browse, while the running assignment stays pinned to its original project. The first release is designed for one Linux host and one active coding task at a time.

## Project status

Outpost is in active development. The source build, local preview, repository onboarding, Queue, Work, History, Codex worker, validation, and delivery paths are implemented. The [v0.1.0-rc1 prerelease](https://github.com/felixlheureux/outpost/releases/tag/v0.1.0-rc1) provides checksum-protected Linux assets and has been installed on the configured host. Clean-host and complete owner-workflow acceptance remain outstanding; see [installation status](INSTALL.md) and [OP-016](https://github.com/felixlheureux/outpost/issues/16). GitHub Issues hold current task requirements; files under [`tasks/`](tasks/) are planning snapshots.

## Architecture and contributing

Outpost uses Go and Echo for the API and embedded UI, React and TypeScript for the interface, SQLite for local state, native Git for checkouts and delivery, and authenticated `gh api` for GitHub. The web and worker processes have separate user-systemd lifecycles; Tailscale provides the private operator boundary.

Read [AGENTS.md](AGENTS.md) before implementing an issue. The [technical specification](Outpost-Technical-Specification.md) defines the product contract, [architecture notes](docs/architecture.md) describe current boundaries, and the [interactive UX reference](Outpost-UX-Reference.html) shows the intended density and flows. The [live queue](https://github.com/felixlheureux/outpost/blob/outpost/queue/queue.json) is on its dedicated planning branch.
