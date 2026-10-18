# Install and onboard Outpost

**Status:** The source-build/local-preview instructions below work. A local-binary `outpost install --prepare` path prepares a private web service without changing an existing Serve route. The installer detects missing Debian/Ubuntu host tools and installs them through scoped apt or official Tailscale/Codex scripts; sign-in remains an explicit user step. Add repository creates a managed checkout, connects GitHub Issues, and initializes or reuses the planning queue through the host `gh` login. An owned worker can execute an assigned task through Codex App Server, run configured local checks with bounded repair, and use selected Git delivery with recoverable GitHub reporting. The private browser Start control is implemented. The full configured private host and owner-device journey still needs acceptance; public release assets are pending.

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

`scripts/install.sh` accepts a locally built binary now. Its release-download path expects future `outpost-linux-amd64` or `outpost-linux-arm64` assets and matching `.sha256` files; no release asset is currently published, so that path must not be reported as working until OP-016 publishes and verifies it. The canonical configuration entry point is:

```sh
./outpost install
```

For a local build on this host, prepare the web service while retaining the current Serve route:

```sh
make build
scripts/install.sh --binary "$(pwd)/bin/outpost" --prepare
```

This detects missing Git, `gh`, curl, Tailscale, Codex, and boot lingering, prints each scoped host action, and installs missing tools by default. `--install-deps=false` prints the needed actions and stops without installing them. Existing tools are reused. You must complete any missing Tailscale, Codex, or GitHub sign-in as the service user and rerun the same command. It installs the binary, private paths and user web unit, applies idle-safe migrations, and checks the loopback API. It does not start coding work or claim an owner-device HTTPS check. The final cutover uses `outpost install --replace-serve-from http://127.0.0.1:OLDPORT` after the replacement workflow is ready; the old root proxy must match exactly. Do not use that flag for an unrelated route.

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

Outpost currently supports `codex-cli 0.163.0` through the official Codex
App Server stdio protocol. Install the official standalone Codex binary as the
service user, then run `codex --version`, `codex login`, and `codex login status`.
`outpost doctor` rejects a missing executable, a different version, or missing
sign-in with a remedy. The tested x86_64 Linux standalone binary is statically
linked and has no Node runtime requirement. A separate npm-based Codex
distribution may require Node; this compatibility contract covers the
standalone binary. The embedded Outpost dashboard does not run a Node server.

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
outpost stop --run-id RUN_ID
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

Open **http://127.0.0.1:8080** on the same machine. This is a local preview, with no production identity. The Go executable serves the actual built UI/API without Vite or Node. State defaults to `$XDG_STATE_HOME/outpost` or `~/.local/state/outpost` with private permissions; `--state-dir /absolute/private/path` overrides it for both migrate and serve. `serve` never silently migrates. To develop with hot frontend refresh, use `make dev` and open http://127.0.0.1:5173.

`./bin/outpost doctor` checks installed Git, existing GitHub/Codex/Tailscale access, the user systemd bus, boot lingering, and the configured private Serve route. Optional `--repo https://github.com/owner/repo` checks repository read access. These checks do not install anything, prove write permissions, configure Serve, or establish worker readiness. No authentication is needed to build or view the local empty shell.

Do not expose development mode through a proxy or public listener. Private `serve`, a preparation-stage `install`, read-only `status`, and a guarded `stop --run-id` are available. The owned worker can run a pinned Codex assignment through validation, selected Git delivery, and local completion. Real account and owner-device acceptance still require a configured private host journey. See docs/delivery.md for reporting and recovery. The built binary is the installer input for OP-002. See docs/development.md for checks and docs/dependencies.md for the sign-in contract.

### Manually exercise the private request boundary

This is an integration path for the access boundary, not the completed installation flow. Run as the eventual service user after Tailscale sign-in. Choose the actual node DNS name from `tailscale status --json` and the exact allowed Tailscale user login. Before creating the route, inspect `tailscale serve status --json`; preserve any existing HTTPS port 443 route. If that port is already occupied, stop and choose a host/port strategy rather than replacing it.

Create `~/.config/outpost/config.json` in a private directory, with mode 0600, replacing the example values:

```json
{"origin":"https://host.tailnet.ts.net","allowed_login":"owner@example.com"}
```

With no conflicting port 443 route, configure only this private Serve mount:

```sh
tailscale serve --bg --https=443 http://127.0.0.1:8080
./bin/outpost migrate
./bin/outpost doctor
./bin/outpost serve
```

Normal `serve` requires the configured DNS name to match this running Tailscale node. `doctor` checks the Serve root and Funnel state. Requests need the exact allowed `Tailscale-User-Login` header from the local Serve proxy; mutations also need the exact HTTPS Origin. This trust boundary includes local processes on the host, so keep the host and service user private. Verify the actual URL from an allowed tailnet device; this document's example is not an installed URL.

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
