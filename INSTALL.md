# Install and onboard Outpost

**Status:** The source-build/local-preview instructions below work for the starter. Release assets, the guided installer, private Tailscale identity and worker are still being built. The release-install sections are the implementation contract for issues #2, #8 and #16; their planned commands are not available yet.

## Choose release installation or source build

| Path | What you need | What it provides |
| --- | --- | --- |
| Release, normal operator path | Supported Linux/systemd host; installable host prerequisites; one-time Tailscale/Codex/GitHub access | One executable containing the dashboard UI and migrations, plus configured user services |
| Build from source | Supported pinned Go/Node/package-manager toolchains, source checkout, build instructions established in OP-001 | The same executable, which is then passed to the normal installer |

The dashboard's release UI does not require a Vite/Node server or a Go compiler. Git, Tailscale, systemd, and the selected Codex distribution remain real host prerequisites. A provider distribution or a repository's build may independently need another runtime; doctor must say which component needs it.

The initial automated package path targets Debian/Ubuntu-family Linux with systemd. The release must list the exact tested distro versions and architectures. Unsupported hosts get explicit manual prerequisites or an unsupported result, never a misleading successful installation.

## Normal release installation: target flow

### 1. Obtain the executable

Use the project's actual release assets for your Linux architecture, or its delivered `scripts/install.sh` bootstrap. The bootstrap must verify the published checksum and call the canonical installer. It must support an explicitly supplied locally built binary so installation can be tested before the first public release exists.

No download URL, artifact name, or bootstrap flag is invented in this starter. OP-002/016 must put the real verified invocation here when those artifacts exist. The canonical configuration entry point is:

```sh
./outpost install
```

### 2. Let the installer inspect and install prerequisites

The installer must inspect before changing anything, reuse compatible tools, and show the missing prerequisites and scoped installation action:

- Git, official GitHub CLI (`gh`), and required system packages.
- User systemd manager and boot/linger requirements for the selected installation user.
- Tailscale CLI/daemon and available private Serve route.
- Supported official Codex distribution and its actual runtime dependencies.
- Writable private binary/config/state/workspace paths.

Package-manager actions may need local elevated privileges. Only those actions use them; Outpost web/worker services run as the chosen ordinary user. A failed package install stops with the actual error and a manual remedy. Re-running continues/reconciles existing setup without replacing working tools or resetting state.

### 3. Complete one-time access

The installer guides these human steps only when they are missing:

1. Sign the host into the intended Tailscale network and select the allowed operator identity.
2. Authenticate the actual Codex installation using its supported flow.
3. Reuse the host’s authenticated GitHub CLI, or guide `gh auth login --hostname github.com --web --git-protocol https` followed by `gh auth setup-git`. Verify repository, issue and planning-file access. No copied PAT, GitHub OAuth app registration, or Outpost account is required. Preserve an existing SSH configuration unless the operator chooses to change it.

Already valid sessions are reused. Tokens/passwords are not written into public configuration output, URLs, process arguments, or logs. The ChatGPT GitHub connection used for remote reviews is distinct from the host credentials used by Outpost.

Missing/expired access is a resumable setup/doctor result, not proof that installation or a project is ready.

### 4. Configure and start private services

The installer creates private XDG paths, installs the executable and user service, applies idle-safe migrations, and configures a Tailscale Serve route without replacing unrelated routes. It verifies allowed identity and the actual UI/API address.

Default layout:

| Content | Location |
| --- | --- |
| Configuration | `~/.config/outpost/config.json` |
| Database and retained artifacts | `~/.local/state/outpost/` |
| Managed repository checkouts | `~/.local/share/outpost/workspaces/` |
| User service | `~/.config/systemd/user/outpost.service` |
| Executable | Selected user binary directory; the installer prints the actual path |

The installer finishes by printing the verified private HTTPS dashboard URL and concise diagnostics. It must not print a generic example URL and claim that is the installed address.

The expected operational commands, once implemented and installed, are:

```sh
outpost doctor
outpost status
```

### 5. Add your first repository

Open the printed dashboard URL from a device on the tailnet. With no projects, choose **Add repository** and paste the GitHub repository URL. Outpost clones onto the Linux host, displays real inspection/clone/configuration progress, reuses or initializes its planning source, and opens that project's Queue.

The normal path requires no SSH directory preparation or local checkout path. Existing checkout registration is an advanced option. A failed source phase retains a successful clone and offers retry of that phase. Existing planning content and main-branch instructions are preserved.

### 6. Add another project and start work

The project menu includes **Add repository** from the first project onward. Add another URL and switch between separate queues, settings, history, conversations, and drafts. Returning reuses each checkout.

Adding a repository authorizes setup; **Start** authorizes coding. Check effective agent/model/permissions, checks, delivery, and task budget before the first run. There is one global coding execution slot. Browsing B while A executes leaves A pinned in the global run bar. Stop/finish and safely settle A before starting B; unfinished A remains preserved in its own checkout.

## Build and install from source

Use Go **1.27.2**, Node **24.19.0**, npm **11.x**, Git and Make. Versions are pinned in go.mod, .node-version and the package lockfile. Python 3 and a Playwright browser are only needed for the browser smoke check. sqlc **1.31.1** is needed only when changing SQL; generated Go is committed.

```sh
git clone https://github.com/felixlheureux/outpost.git
cd outpost
make setup
make build
./bin/outpost migrate
./bin/outpost serve --dev
```

Open **http://127.0.0.1:7331** on the same machine. This is a local preview, with no production identity. The Go executable serves the actual built UI/API without Vite or Node. State defaults to `$XDG_STATE_HOME/outpost` or `~/.local/state/outpost` with private permissions; `--state-dir /absolute/private/path` overrides it for both migrate and serve. `serve` never silently migrates. To develop with hot frontend refresh, use `make dev` and open http://127.0.0.1:5173.

`./bin/outpost doctor` checks installed Git plus existing GitHub/Codex/Tailscale access and reports missing steps. Optional `--repo https://github.com/owner/repo` checks repository read access. These checks do not install anything, prove write permissions, configure Serve, or establish worker readiness. No authentication is needed to build or view the local empty shell.

Do not expose development mode through a proxy or public listener. Production `serve`, `install`, worker and status are deliberately unavailable until OP-002 implements the private boundary and lifecycle. The built binary is that task’s installer input; a separate source-only service architecture is unnecessary. See docs/development.md for checks and docs/dependencies.md for the sign-in contract.

## Diagnostics and common setup problems

| Symptom | Required explanation/remedy |
| --- | --- |
| No user systemd manager or boot persistence | State the missing user-session/linger setup and the exact scoped host action |
| Tailscale signed out or route unavailable | Guide sign-in or select a nonconflicting route; preserve unrelated routing |
| Codex absent, unsupported, or signed out | Identify required distribution/version/authentication; distinguish fixture support from actual readiness |
| Public clone works but queue writes fail | Configure host GitHub issue/contents access; retry source setup without recloning |
| Private repository inaccessible | Check the selected host credential and repository access; do not expose the credential |
| Clone interrupted or disk full | Show the owning setup operation and failed stage; reconcile before retry; preserve registered workspaces |
| Existing queue is invalid/inaccessible | Link the source/reason and offer repair/retry; never silently overwrite it |
| Another coding task is running | Open that run or safely stop/finish it; project selection itself does not transfer ownership |
| No release asset or unsupported platform | Report the real missing artifact/support limit; offer verified source/manual instructions if available |

Closing a browser does not stop a coding run. A web restart must not kill an independently supervised worker. A disconnected stream is not proof that execution stopped.

## Reinstall and update

Re-running installation must preserve configuration, routes, identities, projects, queues, history, and unfinished work. It reconciles completed installation phases rather than deleting and rebuilding state.

For a binary/schema update: finish/stop current work, verify all owned worker/check processes are settled, back up the required state, replace the binary atomically, apply supported migrations, restart the web service, and run doctor. Updating while owned work is active is outside the first-release update contract.

## Backup and restore

Back up the SQLite database consistently together with referenced artifacts, configuration, and managed checkouts containing local-only changes/commits. A repository on GitHub does not contain your unpushed files. Respect WAL/maintenance rules; copying only an active main database file is insufficient.

Restore on an idle installation, preserve the previous state until verified, and check schema/identity/ownership plus referenced artifacts. Do not reclone over restored unfinished work. Keep provider/Git/Tailscale credentials under their existing private owners rather than dumping their stores into a public archive.

## Removal boundaries

Stopping/removing Outpost services is separate from deleting data. Retain project checkouts/history/configuration by default and identify any configured Serve route precisely before changing it. Never reset unrelated Tailscale routes or recursively delete user work as an ordinary uninstall step. OP-002/016 must document actual supported removal commands when implemented.
