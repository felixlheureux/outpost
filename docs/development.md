# Develop Outpost

Use Go **1.27.2**, Node **24.19.0**, npm **11.x**, Git and Make. The committed
Go module and npm lockfile pin dependencies. The Node version is in .node-version.
No account login is needed to build, test or view the local empty shell.

```sh
make setup
make dev
```

Open http://127.0.0.1:5173. Vite proxies API requests to Go on 127.0.0.1:7331.
The dev script builds an asset-free Go binary, migrates disposable private state,
and cleans up its own processes on exit. Adding a repository uses the host's
signed-in `gh` and a managed checkout below the configured workspace directory.
Frontend edits refresh automatically;
restart make dev for Go changes. Set `OUTPOST_DEV_STATE_DIR` to a private absolute
directory to keep development state. Never use an active installation’s state.

## Build the executable

```sh
make build
./bin/outpost version
./bin/outpost migrate
./bin/outpost serve --dev
```

Open http://127.0.0.1:8080. Vite/Node is not serving this dashboard. The build
first creates web/dist, then compiles with CGO disabled and embeds UI/migrations.
Missing web/dist fails normal Go compilation; no placeholder assets are committed.
`make build VERSION=your-label` sets the version. Override `GO` if required.

`make release VERSION=vX.Y.Z` builds checksum-protected static Linux amd64 and
arm64 executables plus the maintenance backup script under `dist/`. A pushed
`v*` tag runs `.github/workflows/release.yml` and publishes those assets after
the source gates pass. The local cross build verifies arm64 compilation; it does
not prove arm64 execution or a clean-host install.

The [v0.1.0-rc4](https://github.com/felixlheureux/outpost/releases/tag/v0.1.0-rc4)
asset has been installed on the configured host; a separate clean-host install
and some owner-device controls remain unverified. See the current
[release evidence](release-evidence.md) and [installation guide](../INSTALL.md).

`--state-dir /absolute/private/path` is a persistent flag for migrate/serve. State
defaults to `$XDG_STATE_HOME/outpost` or `~/.local/state/outpost`. `serve` checks
schema compatibility without migrating. Stop services before explicit upgrades.
The local preview uses explicit unauthenticated loopback development mode.
Normal `serve` checks the configured Tailscale node, allowed login and origin;
the installer separately checks the Serve route. See INSTALL.md for the private
integration path. The guided installer preparation is available through
`scripts/install.sh --binary` and `outpost install --prepare`. Missing supported
host tools are installed by default. The owned worker can run a pinned assignment
through the installed Codex App Server. Configure a default project profile with
`outpost checks set --project ID --file /absolute/checks.json`. The CLI `outpost run --project ID --model
MODEL --reasoning EFFORT` reserves a run and launches its worker; choose a model
and effort returned by the project's `/api/v1/agent/capabilities` endpoint.
Provider handoff proceeds through runner checks, selected Git delivery, local
settlement, and independent GitHub result reporting. See `docs/delivery.md`.
While a run exists, Work shows retained activity, live turn instructions,
stop/continue receipts, and current checkout changes. See `docs/activity.md` for
stream and artifact bounds and `docs/work.md` for the control contract.

## SQL and route generation

Install **sqlc v1.31.1** from its official release or, with Go available:

```sh
go install github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1
make generate
```

Ensure the executable is on PATH, or pass `SQLC=/absolute/path/sqlc`. Edit
migrations/queries, regenerate and commit internal/store/dbgen together. Use a
new numbered migration for an already delivered schema. Ordinary builds use
committed output and do not need sqlc. Do not edit generated Go manually.
Vite’s Router plugin generates web/src/routeTree.gen.ts from web/src/routes;
commit that tree too. `npm --prefix web run build` generates before typecheck.

## Checks

```sh
make format
make generate                         # when SQL changed; CI checks freshness
make check                            # builds binary/UI; typecheck; format/vet/tests; Biome
cd web
npx playwright install chromium      # one-time test browser installation
cd ..
make smoke                            # built binary, disposable state
```

Python 3 is needed only by the disposable smoke-server helper. CI installs the
browser’s OS dependencies too. `OUTPOST_TEST_CHROMIUM=/absolute/chromium` can
select an existing test browser if the standard download is unavailable.
The browser journey covers empty state, a simulated setup rejection with preserved
URL, navigation/refresh, mobile overflow and API/assets 404. A separate gated Go
integration test uses the host `gh` login to clone a real disposable repository
and replay its setup. It may initialize that repository's planning branch, so it
requires both `OUTPOST_GH_REPO_TEST_URL` and `OUTPOST_GH_QUEUE_TEST_WRITE=1`.
Go tests cover
read-only schema checks, real migrations, future-schema refusal, SQLite pragmas,
real disposable Git, controlled gh argv/error redaction and HTTP boundaries.
The gh executable in tests is a fixture, not GitHub-account acceptance.

Format/generate before the final gate. Run focused checks during development.
Do not repeat an unchanged full suite after docs, status, queue or evidence-only
closeout. Code, relevant config, fixtures and contracts invalidate affected checks.
CI also checks committed generated-source drift.

## Authentication and production acceptance

`./bin/outpost doctor` inspects existing tools/logins, the user systemd bus,
lingering, and the configured private Serve route; `--repo URL` adds a
repository read check. Use the official GitHub CLI browser login and credential
helper (docs/dependencies.md). Do not require API-key copying or a new OAuth app.
Setup must run as the eventual service user. Services cannot open an interactive
login or silently change accounts.

The opt-in `OUTPOST_REAL_CODEX_TEST=1 go test ./internal/codexadapter
./internal/providerwork` runs real Codex turns in disposable repositories and
needs an authenticated service-user account. The owned provider test also
verifies the SQLite receipt and leaves the assignment awaiting validation.
Ordinary tests use a protocol fixture. Set `OUTPOST_REAL_CODEX_CONTROL_TEST=1`
to run the separate real steering, interruption, process settlement, and
saved-session resume acceptance in a disposable repository. Set
`OUTPOST_REAL_CODEX_WORK_CONTROL_TEST=1` for the full Outpost SQLite receipt,
provider control, and linked continuation acceptance in another disposable
repository.
Validation uses operator-configured profiles, retained check receipts and bounded
repair; see `docs/validation.md`. `outpost checks recover --run-id ID` can
reconcile a terminal check after the worker stops. A local pass does not settle
Git delivery. The private Start panel and CLI capture the delivery policy;
`outpost run --delivery local_changes|commit|push --review none|required`
sets defaults, and `--close-issue` opts into issue closure after reporting.
These tests do not establish production user-systemd ownership, Tailscale private
identity, GitHub write permissions or phone acceptance. Those need separate host,
account and device evidence.
