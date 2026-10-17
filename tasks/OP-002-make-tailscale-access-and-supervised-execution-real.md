---
planning_id: OP-002
status: published
order: 2
depends_on: ["OP-001"]
source: bootstrap-handoff
github_issue: 2
---
# OP-002 — Install Outpost and prerequisites with private access and supervision

**Dependencies:** #1 (OP-001)  
**Specification:** §§6, 12, 15, 22–23, 25.  
**Acceptance owners:** AC-01–04, AC-12–17, AC-25–26 at supervision level; AC-61–64 and AC-66.

**Canonical task:** https://github.com/felixlheureux/outpost/issues/2  
**Repository copy:** Published planning snapshot; update the issue for current requirements.

### Outcome

The owner installs Outpost through a documented guided flow that detects/installs supported host prerequisites, establishes real access, configures private user services, and prints a verified dashboard URL. Independently supervised execution survives web/browser interruption, and ownership uncertainty cannot launch a second writer.

### Implementation requirements

1. Add installation/configuration and user-systemd integration. Separate the long-lived web unit from uniquely named run worker units invoking the same executable with a run identity.
2. Inspect current Tailscale status/Serve configuration and select a route without replacing unrelated routes. Bind Echo to loopback. Print the actual private HTTPS address and allowed identity after verified setup.
3. Trust Tailscale identity headers only through the configured local access boundary; deny missing/disallowed identities across HTML, API, SSE, and artifacts. Add exact-origin/CSRF protections to mutations. Do not open a public port or enable Funnel.
4. Persist run reservation/launch intent before D-Bus effects. Record unique run token/unit identity and check worker ownership before repository or agent effects. Do not hold a database transaction during systemd calls.
5. Reconcile missing/lost launch acknowledgements by observing the recorded unit and process identity. Replayed starts return the existing receipt; duplicate worker claims fail safely.
6. Define worker/check process-group settlement and OS workspace-lock lifetime. An owned surviving child retains exclusion even if its parent disappears. Normal recovery uses observed ownership, never deleting a lock file.
7. Add install/status/stop diagnostics for absent user bus, disabled lingering, unavailable Tailscale, unavailable CLI prerequisites, unwritable directories, and unknown owned processes. Keep requested host changes explicit and narrow.

### Installation and first-run contract

Deliver a working scripts/install.sh release bootstrap and one canonical outpost install implementation, together with a complete operator INSTALL.md. The bootstrap obtains the correct supported Linux/architecture executable from this repository's real release assets, verifies its published checksum, and hands configuration to outpost install; it must not duplicate service/configuration logic. Support a supplied local built binary for development/offline bootstrap acceptance before public releases exist.

Automate prerequisite inspection and installation for the first supported Linux target (implementation default: Debian/Ubuntu family with systemd; record verified versions/architectures). Check Git, Tailscale daemon/CLI, user-systemd support and required system packages, the supported Codex distribution, writable paths, and source/provider access. Show what is missing and the exact package action; use host package-manager privileges only for those actions. Already installed compatible tools are reused. The web/worker run as the chosen unprivileged user. Unsupported/failed package paths stop with an explicit manual remedy rather than a false success.

Use an official supported Codex installation method pinned/verified with OP-008. The dashboard's release install requires neither Go nor a Vite/Node dashboard server. A provider distribution may independently need Node; detect/document that rather than conflating frontend build tooling with runtime requirements. Do not install every possible target repository's toolchain globally.

Guide the one-time human authentication boundaries: Tailscale login/allowed identity, Codex sign-in, and the host GitHub credential reference/required repository access. Detect existing authentication and reuse it. Do not fake successful sign-in, copy tokens from the browser, expose credentials in logs/argv/config output, or auto-accept unrelated permissions. Missing authentication remains resumable setup; repeated installation continues from known completed phases.

Create private XDG config/state and managed-workspace directories, install the executable and user units, deliberately handle boot/linger prerequisites, apply idle-safe migrations, preserve existing Serve routes/settings, and verify the actual HTTPS UI/API boundary. Finish by printing the dashboard URL plus concise doctor/status results; the first UI should show Add repository when no projects exist. Install is idempotent and preserves project/queue/history/unfinished work. INSTALL.md must also cover source install, upgrade, backup/restore, failure recovery, and removal boundaries.

### Failure and edge cases

Browser disconnection proves nothing about worker state. A successful systemd job acknowledgement does not prove task completion. A web restart must not kill the worker; a VM reboot must not silently resume an arbitrary effect. Keep reservation until known worker/check ownership is reconciled.

### Acceptance criteria

- [ ] A real user-systemd fixture survives web-service restart and can be observed/stopped afterward.
- [ ] Concurrent/replayed starts establish one reservation and one worker.
- [ ] Lost launch acknowledgement attaches to the existing unit rather than launching twice.
- [ ] A surviving check fixture prevents another writer and exposes a relevant stop action.
- [ ] Existing Serve routes survive installation; allowed/private access succeeds and denied identity/origin controls fail.
- [ ] Closing the browser leaves execution unaffected.

- [ ] scripts/install.sh and outpost install form a documented working path using a release asset or explicitly supplied local binary; no duplicate installer implementation exists.
- [ ] A clean supported Linux host detects missing prerequisites, performs the documented scoped dependency installation, and reuses already compatible tools.
- [ ] Tailscale/Codex/GitHub authentication is guided, resumable, and truthfully verified; missing access is not reported ready.
- [ ] A second install preserves existing config/Serve routes/projects/data/unfinished work and prints the same usable dashboard address.
- [ ] INSTALL.md covers release versus source prerequisites, exact commands, one-time auth, first Add repository, doctor/troubleshooting, upgrade/backup/restore, and safe removal boundaries.
- [ ] The release UI runs without Go or a dashboard Node/Vite process; any independent provider runtime requirement is explicitly identified.

### Verification and handoff

Use deterministic local process fixtures for ownership races, then a real Linux user-systemd/Tailscale smoke journey. Label unavailable host checks explicitly. Handoff exposes the supervisor interface, reservation lifecycle, process identity receipt, and current liveness projection for the scheduler and UI.


### Starter integration and simple authentication

Read docs/starter-handoff.md, docs/architecture.md and docs/dependencies.md. The embedded Go/React foundation already exists. Extend it rather than regenerating the application. Use native Git for checkouts and the existing internal/githubcli adapter for authenticated gh API calls. Normal installation reuses `gh auth login --hostname github.com --web --git-protocol https`, `gh auth setup-git` where appropriate, Codex account sign-in, and Tailscale sign-in. Do not require copied API keys, a PAT, GitHub OAuth app registration, or an Outpost account. Reuse valid sessions and preserve existing SSH choices. Run setup as the eventual ordinary service user; verify that its credential store works in that service environment. Human sign-in remains a guided resumable step. Do not try to open login dialogs from a background HTTP command.
