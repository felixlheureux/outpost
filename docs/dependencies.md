# Dependency decisions

The executable and SPA are small enough to use direct libraries and explicit
configuration. Exact Go/npm versions are in the committed manifests and lockfiles.
Use the tested toolchains in `docs/development.md` rather than copying examples
from older Echo or Tailwind major versions.

| Need | Use now | Why |
| --- | --- | --- |
| HTTP | Echo v5 | Small routing and HTTP adapter surface |
| CLI | Cobra | One executable with understandable operational commands |
| Local state | database/sql + modernc SQLite | No database daemon or CGO toolchain |
| Schema / SQL | Goose library + sqlc generator | Embedded upgrades, explicit SQL, typed committed query code |
| Logs / assets / processes | Go standard library | slog, embed, os/exec already cover the need |
| Checkout operations | Native `git` | Full Git behavior and existing credential helpers |
| GitHub API / login | Official `gh` CLI | Browser/device login, existing credentials, no token-copy setup |
| UI | React + TypeScript + Vite | Static build embedded in Go; no runtime Vite server |
| Routes | TanStack Router | Typed routes/URL state; file routes with build-time generation |
| Server state | TanStack Query | Cache, fetch lifecycle and explicit mutation invalidation |
| Response validation | Zod | Check untrusted JSON once at the HTTP boundary |
| Components | Tailwind v4 + shadcn/Radix + Lucide | Owned accessible primitives and consistent design tokens |
| Frontend checks | Biome + TypeScript | One formatter/linter plus real type checking |
| Integration evidence | Go tests + Playwright | Disposable SQLite/Git, HTTP boundaries, actual embedded-binary journey |

## Deliberately deferred

Octokit belongs in a JavaScript GitHub integration; this backend does not need it.
`go-github` would be reasonable for a long-lived SDK transport or a multi-user
service, but currently duplicates gh authentication/integration. Reconsider if
measured process overhead or API needs justify it; keep that change behind the
GitHub adapter, not scattered through handlers.

No ORM, Viper, Redis, message broker, dependency injection container, separate
OAuth app, frontend state framework or mandatory Docker setup. Add validator,
React Hook Form, D-Bus/systemd libraries, SSE helpers or component primitives when
their concrete feature needs them. Ordinary form constraints and explicit Go
validation suffice until then. Do not add unused packages for future tasks.

## Setup without copied API keys

Release installation should be: obtain Outpost → guided host checks → complete
missing GitHub/Codex/Tailscale sign-ins → open the verified URL → Add repository.
Reuse valid sessions. GitHub's normal path is:

```sh
gh auth login --hostname github.com --web --git-protocol https
gh auth setup-git
```

These are official CLI flows, not an Outpost API key or GitHub app registration.
An existing SSH Git setup can remain; the installer must not silently replace it.
Codex uses its own supported account sign-in and Tailscale its own device sign-in.
Account authorization still requires the user, and private-repository/organization
policies may require additional approval. There is no Outpost account to create.
Do not open login flows from background HTTP/worker commands or log tool output
containing credentials. The OP-002 installer now installs missing supported host
tools and preserves existing CLI sign-ins; real project and provider readiness
remain separate from installation.

## Updating

Update direct dependency versions deliberately, commit `go.sum` and
`web/package-lock.json`, and run affected checks. Keep Router's build plugin
compatible with its runtime. Update sqlc's pinned generator and regenerated source
together. Copy shadcn components only when used and preserve their upstream license
notice in `THIRD_PARTY_NOTICES.md`. Do not commit node_modules or web/dist.
