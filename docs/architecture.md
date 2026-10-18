# Starter architecture

Outpost is one Go executable with an embedded React SPA and SQLite migrations.
The starter implements the local shell and persistence boundary. The current
implementation adds a pinned Tailscale operator boundary, a preparation-stage
private web service, worker ownership fixtures, managed GitHub cloning and an
issue source projection with durable create/edit receipts, and a GitHub planning
queue projection with durable edit receipts, the Queue/Inbox workspace, a
durable scheduler with fresh-source admission and an eligibility preview, and
a Codex App Server adapter with provider attempt and live control receipts, and
runner-owned local validation with private check artifacts and bounded repair.
Git delivery receipts, independent GitHub reporting, and a private browser Start
path now connect those checks to local completion.
The complete behavior contract remains the technical specification.

## Follow a request

`cmd/outpost` → `internal/cli` composes SQLite, HTTP and embedded files.
`internal/httpapi` maps HTTP DTOs to direct store calls. SQL lives in `queries/`;
sqlc generates `internal/store/dbgen`. Add a small domain service when an actual
operation coordinates persistence and external effects. Do not create interfaces
or service layers for every table in advance.

| Path | Owns |
| --- | --- |
| `cmd/outpost` | Signals and exit code |
| `internal/cli` | Cobra commands, flags, composition, graceful HTTP shutdown |
| `internal/httpapi` | Versioned JSON contracts, request bounds, route precedence |
| `internal/store` | Private SQLite connection and explicit schema lifecycle |
| `migrations`, `queries` | Authoritative SQL; never edit generated query Go |
| `internal/hostcmd` | Bounded noninteractive subprocess execution without a shell |
| `internal/gitcli` | Native Git checkout facts; extend for clone/delivery later |
| `internal/githubcli` | GitHub URL parsing and authenticated `gh api` metadata |
| `internal/issues` | Bounded requirements parsing, source sync and issue mutation reconciliation |
| `internal/projectsetup` | Durable managed checkout staging, clone and retry |
| `internal/queue` | Planning manifest validation, source sync, edit and inspection receipts |
| `internal/scheduler` | Fresh manifest/dependency observation, eligibility, immutable assignment and fixture worker loop |
| `internal/codexadapter` | Pinned Codex App Server stdio protocol, capability discovery, session controls and structured handoff |
| `internal/providerwork` | Immutable prompt, private bounded event artifact and owned provider receipt transitions |
| `internal/validation` | Captured check plans, candidate identity, owned child processes, terminal reconciliation and repair evidence |
| `internal/delivery` | Candidate-bound local/commit/push reconciliation and GitHub result reporting |
| `activity_events`, `activity_artifacts` | Bounded notifications and allowlisted private output manifest; see `docs/activity.md` |
| `work_messages`, `work_controls` | Scoped live directions, stop and continuation receipts; see `docs/work.md` |
| `internal/supervisor`, `internal/worker` | Recorded worker units, process identity and workspace exclusion |
| `web/assets*.go` | Production embedding; explicit development build variant |
| `web/src/routes` | TanStack Router file routes and route-level composition |
| `web/src/lib/api.ts` | Fetch, explicit error envelope, Zod response validation, Query keys |
| `web/src/components/ui` | Owned shadcn primitives; compose them instead of restyling copies |
| `web/src/components/shell.tsx` | Application chrome and repository draft ownership |

## Data and process boundaries

GitHub Issues own task requirements; the planning branch owns queue order. The
SQLite projects table is the first local projection, not a second task source.
The runs table owns one global execution slot and launch receipts; the separate
reservation state stays held until worker/process settlement. `run_schedules`,
`execution_generations`, `assignments`, and `assignment_attempts` own captured
policy, completion identity, immutable source/baseline snapshots, and attempt
phase receipts. No task rank field is authoritative.
`provider_attempts` records one provider intent per linked attempt, selected
settings, exact thread/turn IDs and a proposed handoff. A completed provider
turn advances the attempt to validation without settling the assignment.
`activity_events` provides replay cursors and small event payloads; business
receipts remain authoritative. Provider output lives under private state and
is exposed only through an artifact ID.
HTTP DTOs omit database internals and host paths.

Use native Git for clone/status/diff/commit/push. Use `gh api` for GitHub Issues,
repository metadata and contents-file SHA operations. Reuse `gh auth login`;
never retrieve a token into Outpost or require a GitHub OAuth app/PAT setup.
The service and interactive setup must use the same ordinary OS user and gh
credential environment. A service that cannot unlock its credential store must
report that limitation. Web-authenticated ChatGPT is a separate client.

Current read adapters run with a deadline, output bounds, no terminal prompts,
and redacted errors. Future mutations need their own persisted intent, stable
identity and reconciliation; do not put a generic retry around a push/issue/create.
Agent/check process supervision is a different boundary: do not reuse the short
read-command helper to implement the worker's durable process ownership.

## Local development and production boundary

Vite serves development assets and proxies the same API paths to Go. Production
builds embed `web/dist`; compiling without those assets fails. `-tags dev` is
an explicit asset-free build for development tests. It is not a release fallback.
`serve --dev` is loopback-only, rejects external Host headers and cross-origin
mutations, and explicitly has no production identity. Tailscale is not configured
by the original starter. Normal `serve` now uses the configured Tailscale node
and login; the route is checked separately so web can start before a Serve cutover.

`migrate` applies embedded Goose migrations; `serve` only checks their version.
Stop all future services before upgrades. SQLite uses WAL, foreign keys, a busy
timeout and one connection; network/process calls stay outside transactions.
State defaults to `$XDG_STATE_HOME/outpost` or `~/.local/state/outpost`, never the
coding checkout. The state directory must be 0700 and database 0600.

## Frontend ownership and design

TanStack Router owns locations and project URL selection. Query owns server
data; the router receives the same QueryClient and preloads through
`ensureQueryData`. Do not create a second route cache of project/run objects.
React owns transient drafts. The Add repository dialog stays mounted at the root,
so closing it or navigating between Queue/Work/History preserves the typed URL.
Issue drafts are keyed by repository and issue in the shell. Queue tab, selection,
and list position are per-repository browser session state. The selected issue's
number, content digest, and local revision are the source identity available for
OP-007's immutable assignment; no start request is sent by the current UI.
There is no Redux/Zustand, SSR framework, or independent frontend backend.

Use the CSS tokens in `index.css` and shadcn variants. Keep the compact sidebar,
quiet workspace header, primary Queue/Work/History and global run bar. Avoid
invented metrics and pretend activity. Desktop content is bounded; mobile turns
the navigation into a top strip. Honor reduced motion and visible focus.

## Next integration points

OP-012 connects runner-verified checks to Git delivery and local completion.
See `docs/delivery.md` for effect and reporting receipts. OP-013 can use these
receipts for History and recovery projections.
The Work activity stream reads real SQLite cursors and provider artifacts;
`docs/activity.md` defines its reset, preview and retention rules. The private
Serve cutover, owner-device acceptance and real GitHub planning-branch write
still require their separate host/account verification.
