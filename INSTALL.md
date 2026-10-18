# Install and onboard Outpost

**Status:** The `v0.1.0-rc5` prerelease has Linux amd64/arm64 binaries and checksums. Its bootstrap installed the verified amd64 asset on the configured host and confirmed the private dashboard. A separate clean-host install remains unverified; the configured host is the deployment target for this release.

## Choose release installation or source build

| Path | What you need | What it provides |
| --- | --- | --- |
| Release, normal operator path | Supported Linux/systemd host; installable host prerequisites; one-time Tailscale/Codex/GitHub access | One executable containing the dashboard UI and migrations, plus configured user services |
| Build from source | Go 1.27.2, Node 24.19.0, npm 11, Git, Make, and a source checkout | The same executable, which is then passed to the normal installer |

The dashboard's release UI does not require a Vite/Node server or a Go compiler. Git, Tailscale, systemd, and the selected Codex distribution remain real host prerequisites. A provider distribution or a repository's build may independently need another runtime; doctor reports which host component is missing.

The automated package path targets Debian/Ubuntu-family Linux with systemd. The configured host was verified on Ubuntu 26.04.1 amd64. The arm64 release is cross-built and checksum-protected; execution on an arm64 host remains unverified. Unsupported hosts receive explicit prerequisites or an unsupported result.

## Normal release installation

### 1. Obtain the executable

Download `scripts/install.sh` from the matching release tag and run it as the service user. It selects `outpost-linux-amd64` or `outpost-linux-arm64`, verifies the matching `.sha256` asset, then invokes the canonical installer. Use `--release v0.1.0-rc5` for the published prerelease; the default uses GitHub's latest stable release, which is not published yet. The bootstrap also accepts an explicitly supplied locally built binary.

```sh
curl -fsSLo /tmp/outpost-install.sh https://raw.githubusercontent.com/felixlheureux/outpost/v0.1.0-rc5/scripts/install.sh
sh /tmp/outpost-install.sh --release v0.1.0-rc5
```

The canonical configuration entry point is:

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

The installer inspects before changing anything, reuses compatible tools, and shows missing prerequisites and scoped installation actions:

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

The installer prints the verified private HTTPS dashboard URL and concise diagnostics for the installed address.

Operational commands are:

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

Do not expose development mode through a proxy or public listener. Private `serve`, `install`, read-only `status`, and guarded `stop --run-id` are available. The owned worker runs a pinned Codex assignment through validation, selected Git delivery, local completion, and independent GitHub reporting. See docs/delivery.md for reporting and recovery. Supply the built binary to `scripts/install.sh --binary` for source installation. See docs/development.md for checks and docs/dependencies.md for the sign-in contract.

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

Re-running installation preserves configuration, routes, identities, projects, queues, history, and unfinished work. It reconciles completed installation phases without rebuilding state.

The current `outpost install` refuses a differing installed binary unless `--upgrade` is supplied with a matching private backup. Finish/stop work and inspect `outpost status` until there is no active or unresolved run. Stop the web service, create the backup below, then run the new release bootstrap with `--upgrade --backup-dir "$BACKUP"` (or use `scripts/install.sh --binary /absolute/path/to/new/outpost` with those flags). The installer checks the backup manifest and idle state before atomically replacing the binary, saves the old executable as `previous-outpost` in the backup, applies supported migrations, starts the user service, and verifies the private URL. An unchanged reinstall remains idempotent. Keep the backup until the new service is verified; if migration fails, inspect the retained backup and previous binary before recovery. Migration refuses a reserved worker/check or an active web unit serving the same state directory; a newer unsupported schema also fails.

## Backup and restore

With all work settled, stop the web service and run the maintenance-only Python 3 backup tool. It refuses an active web service, an active worker unit, or a reserved run; snapshots SQLite through its backup API and copies state artifacts, private config, and managed checkouts. It excludes previous backups, runtime locks, and SQLite WAL/SHM files. Keep the resulting directory private; it may contain source code and logs. A repository on GitHub does not contain unpushed work.

```sh
outpost status
systemctl --user stop outpost.service
python3 scripts/backup.py --output "$HOME/.local/state/outpost-backups/backup-$(date -u +%Y%m%dT%H%M%SZ)"
systemctl --user start outpost.service
```

Create the private `outpost-backups` parent first (`install -d -m 700 "$HOME/.local/state/outpost-backups"`). A published release also includes the checksum-protected `outpost-backup.py` asset, which can be run in place of the source script. Python 3 is required for this maintenance command, not for the dashboard runtime.

Before a live restore, copy the backup's `state` directory to a disposable location and check `PRAGMA integrity_check`, schema version, project rows, and workspace copies. On an idle installation, stop the service, move the existing state/config/workspaces aside, and copy the backup's `state`, `config.json`, and `workspaces` to the absolute paths in `manifest.json` with private ownership/modes. Keep the moved-aside data until the restored installation passes `outpost status`, schema checks, and private URL verification. Do not reclone over restored unfinished work. Codex/Git/Tailscale credential stores remain with their existing owners and are not included in this backup.

## Removal boundaries

Stop any recorded worker through `outpost stop --run-id RUN_ID` and wait for `outpost status` to show no active or unresolved run. Then stop and disable only Outpost's web unit:

```sh
systemctl --user disable --now outpost.service
rm -- "$HOME/.config/systemd/user/outpost.service"
systemctl --user daemon-reload
```

Keep `~/.config/outpost`, `~/.local/state/outpost`, and `~/.local/share/outpost/workspaces` as the default; they contain configuration, receipts, and possibly unpushed work. Inspect `tailscale serve status --json` before removing a route. Remove only the root handler that still proxies to this Outpost installation; leave unrelated Serve routes and Tailscale login intact. Keep the executable and private backup until recovery is no longer needed.
