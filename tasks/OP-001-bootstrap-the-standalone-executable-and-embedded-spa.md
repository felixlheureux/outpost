---
planning_id: OP-001
status: published
order: 1
depends_on: []
source: bootstrap-handoff
github_issue: 1
---
# OP-001 — Bootstrap the standalone executable and embedded SPA

**Dependencies:** None.  
**Specification:** §§5–7, 18–19, 24–25, 30.  
**Acceptance owners:** AC-05–06, foundation for AC-37, AC-61, AC-65.

**Canonical task:** https://github.com/felixlheureux/outpost/issues/1  
**Repository copy:** Published planning snapshot; update the issue for current requirements.

### Outcome

A fresh checkout builds one Go executable that serves the actual React application and same-origin API, creates local state through versioned migrations, and needs no Vite/Node server at runtime.

### Implementation requirements

1. Inspect the target repository first. Establish the Go module and pin compatible Go/Echo/library versions; select versions from current official documentation during implementation rather than copying unverified major-version imports. Record the supported toolchain. Preserve/reconcile the delivered starter `AGENTS.md`, issue templates, queue seed/helper, and setup docs; these assets already exist and should not be replaced with a second convention.
2. Add Echo, Cobra, structured `slog` logging, `modernc.org/sqlite`, Goose migrations, and sqlc generation as needed by the actual code. Keep a direct application/service/store call graph; no dependency-injection container or generic repository framework.
3. Scaffold React, TypeScript, Vite, Tailwind, shadcn, a single router, and TanStack Query. Use consistent product tokens and light/dark support. Provide a real initial shell and empty states rather than placeholder progress.
4. Build the frontend before compiling the embedding package. Embed built assets and migrations. Fail the release build clearly when required frontend output is absent; do not silently embed a development placeholder.
5. Implement correct route precedence: `/api/v1` routes first, real static assets next, SPA navigation fallback last. Unknown API paths and missing asset files return their real 404 status rather than `index.html`.
6. Establish explicit HTTP DTOs/errors, request bounds, request IDs, UTC timestamps, and resource IDs. Keep database records and host paths private to the server.
7. Open SQLite with foreign keys, busy timeout, bounded connections, and short transactions. Apply migrations deliberately; normal web/worker startup must not race to migrate. Add only the tables/columns needed now and extend through later migrations.
8. Provide `serve`, `version`, and basic `doctor` entry points; reserve `worker` wiring for the supervision task. Document development proxying to Echo and the production build sequence.

### Installation and first-run contract

Establish and document actual clean-checkout source-build prerequisites and commands in INSTALL.md and docs/development.md: pinned supported Go/Node/package-manager toolchains, frontend dependency install from the committed lockfile, frontend build before Go embedding, sqlc/generation where required, and the resulting executable. Do not require the operator to guess commands or treat an unimplemented command as runnable. Source toolchains are not mandatory dashboard runtime dependencies. Provide the locally built binary as the input for OP-002 installer acceptance before public release assets exist.

### Failure and edge cases

Unsupported newer schema fails clearly. A half-built frontend cannot pass the packaging check. Direct navigation to a frontend route succeeds; a typo in an API/asset route remains a 404. Runtime state directories are outside the source checkout and receive private permissions.

### Acceptance criteria

- [ ] A clean documented command produces the executable and actual UI assets.
- [ ] Launching that executable without a dashboard Node/Vite process serves UI and API.
- [ ] A direct frontend-route refresh succeeds while missing API/assets remain 404.
- [ ] Fresh migrations apply to a temporary SQLite database and reject an unsupported schema.
- [ ] Typecheck, production build, and a release-binary HTTP smoke check pass.

- [ ] INSTALL.md contains the actual working source-build sequence and clearly separates build tools from release runtime prerequisites.
- [ ] A clean-checkout build yields the binary used by the host installer without a runtime Vite server.

### Verification and handoff

Use one focused routing/migration integration test and one embedded-binary smoke journey. Report executable size and idle memory only if actually measured, with the environment stated. Handoff includes build commands, state-path convention, API-error shape, migration ownership, and where later services plug into the shell.
