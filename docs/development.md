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
and cleans up its own processes on exit. Frontend edits refresh automatically;
restart make dev for Go changes. Set `OUTPOST_DEV_STATE_DIR` to a private absolute
directory to keep development state. Never use an active installation’s state.

## Build the executable

```sh
make build
./bin/outpost version
./bin/outpost migrate
./bin/outpost serve --dev
```

Open http://127.0.0.1:7331. Vite/Node is not serving this dashboard. The build
first creates web/dist, then compiles with CGO disabled and embeds UI/migrations.
Missing web/dist fails normal Go compilation; no placeholder assets are committed.
`make build VERSION=your-label` sets the version. Override `GO` if required.

`--state-dir /absolute/private/path` is a persistent flag for migrate/serve. State
defaults to `$XDG_STATE_HOME/outpost` or `~/.local/state/outpost`. `serve` checks
schema compatibility without migrating. Stop services before explicit upgrades.
The starter supports explicit unauthenticated loopback development only; normal
serve, the operator installer and Tailscale identity are still OP-002.

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
The browser journey covers empty state, deliberate unavailable setup, preserved
URL, navigation/refresh, mobile overflow and API/assets 404. Go tests cover
read-only schema checks, real migrations, future-schema refusal, SQLite pragmas,
real disposable Git, controlled gh argv/error redaction and HTTP boundaries.
The gh executable in tests is a fixture, not GitHub-account acceptance.

Format/generate before the final gate. Run focused checks during development.
Do not repeat an unchanged full suite after docs, status, queue or evidence-only
closeout. Code, relevant config, fixtures and contracts invalidate affected checks.
CI also checks committed generated-source drift.

## Authentication and production acceptance

`./bin/outpost doctor` inspects existing tools/logins; `--repo URL` adds a
repository read check. Use the official GitHub CLI browser login and credential
helper (docs/dependencies.md). Do not require API-key copying or a new OAuth app.
Setup must run as the eventual service user. Services cannot open an interactive
login or silently change accounts.

Tests here do not establish actual Codex App Server compatibility, user-systemd
ownership, Tailscale private identity, GitHub write permissions or phone acceptance.
Those need real host/account/device evidence in their owning tasks.
