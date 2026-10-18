# Working starter handoff

OP-001 is implemented. Read this before continuing with OP-002/003; do not replace
the starter with another scaffold. Exact commands: docs/development.md. Package
boundaries: docs/architecture.md. Library/auth choices: docs/dependencies.md.

## Delivered

- Go 1.27.2, Echo v5.4.0, Cobra, CGO-free SQLite, Goose library and sqlc v1.31.1.
- React/TypeScript/Vite/Tailwind/shadcn, TanStack Router file routes, TanStack Query,
  Zod response validation and light/dark tokens. Exact npm versions are locked.
- CLI `version`, explicit `migrate`, loopback `serve --dev`, read-only `doctor`.
- Real SQLite project-list projection and bootstrap capabilities; no seed data.
- Embedded SPA, lazy repository form, API/assets 404s and direct-route refresh.
- Repository URL draft remains owned by the root across close/reopen/navigation.
- Native Git and gh metadata adapters: bounded noninteractive argv, timeouts and
  redacted failures. Existing credentials stay with their official tools.
- Source setup/build/check scripts, pinned-action CI workflow, short docs and
  preserved original spec, templates, issue map and planning branch.

HTTP errors use `{ "error": { "code", "message", "request_id" } }` for application
responses. The development access boundary can reject requests before routing.
Project DTOs contain public identity/setup fields, no host paths. Timestamps use
UTC; future local resources should use the spec's prefixed random IDs.

## Verified locally

Linux/amd64, Go 1.27.2, Node 24.19.0, npm 11.9.0:

- Fresh source copy without node_modules/web/dist: dependency install, frontend
  production build/typecheck, CGO-disabled embedded executable build.
- Go formatting, vet and tests: actual SQLite migrations/pragmas, read-only schema
  check/future-schema refusal, real disposable Git, fixture gh boundary, HTTP
  routing and origin/Host rejection. SQL regeneration is unchanged.
- Biome with recommended rules; generated Router tree remains stable.
- Production compilation rejects absent UI assets; normal serve and public
  development listeners are refused. The formatter gate fails if its tool is absent.
- Actual built-binary browser journey: empty Queue, rejected setup preserves URL,
  Work/History, refresh, mobile layout without overflow, API/assets 404.
- Desktop/mobile screenshots inspected. Development Vite proxy was also exercised.

The browser smoke used Chromium 153 through OUTPOST_TEST_CHROMIUM because this
environment blocked the default Playwright browser download. CI is configured
to install Playwright's normal browser; a hosted CI run is separate evidence.
The stripped CGO-free binary measured **12,783,776 bytes (12.2 MiB)** here. This is
the dashboard starter, not a total installation/agent memory or runtime benchmark.
Vite reports a 527.72 kB main minified JS chunk (167.11 kB gzip estimate); it still
emits its default 500 kB warning. The form and page routes are split. Treat future
bundle growth deliberately; do not hide the warning with a higher threshold.

## Still deliberately unavailable

POST /api/v1/projects returns 501 `feature_unavailable`; the form explains this
and retains input. No managed clone, source initialization, task queue execution,
Codex session, worker, installer or Tailscale identity is claimed. Normal `serve`
refuses to start until the private production boundary is implemented. Fixture
tests do not prove GitHub account access, Tailscale/systemd/Codex or phone acceptance.

## Continue here

OP-002 receives bin/outpost as its locally built installer input. Add the canonical
installer, real private identity and independent web/worker ownership. Preserve
explicit idle-safe migration ownership. Reuse valid **GitHub CLI, Codex and
Tailscale sign-ins**; normal setup must require no copied API keys/PAT, OAuth app
registration or Outpost account. Run interactive setup as the service's OS user
and verify its eventual credential-store access. Guide only missing human sign-ins.

OP-003 extends projects with durable managed-clone/source setup and actual project
selection. Replace the 501 boundary and enable the bootstrap capability only when
the real flow exists. Keep shell-owned unsent drafts and the compact navigation.
Add a new migration and regenerate SQL, rather than changing the delivered schema.
All later tasks retain their original authority, ownership and acceptance contracts.
The live planning queue is unchanged; completed work is skipped, not removed.
