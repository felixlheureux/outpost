# Outpost — Implementation Tasks

**Revision:** 0.4 · October 17, 2026 · Published to felixlheureux/outpost
**Target:** First usable personal release on a Linux host over Tailscale  
**Companion specification:** [Outpost-Technical-Specification.md](Outpost-Technical-Specification.md)  
**Interactive design reference:** [Outpost-UX-Reference.html](Outpost-UX-Reference.html)

## How to use this backlog

Read the specification's confirmed decisions, authority model, and acceptance strategy before implementation. Then work on the next dependency-ready task in the order below. Each task is an outcome with concrete acceptance, not a suggestion to build every future extension.

The task files in `tasks/` retain stable `OP-xxx` planning IDs. All 16 tasks are now published in [felixlheureux/outpost](https://github.com/felixlheureux/outpost/issues); `issue-map.json` records their actual issue identities. The live `queue.json` is on `outpost/queue` and uses real issue numbers. These repository task files and this document are published reference snapshots; update GitHub Issues for current requirements and the planning branch for current order.

If the repository already contains implementation, inspect it and its `AGENTS.md`/documentation first. Reuse working code, mark demonstrated acceptance, and implement the remaining behavior. This package does not claim every component must be rewritten or that an empty repository exists.

## Starter implementation status

OP-001 is implemented and locally verified; see [docs/starter-handoff.md](docs/starter-handoff.md). Continue with OP-002. The shell uses TanStack Router/Query; native Git and authenticated gh reuse existing logins. Normal installation must not require copied API keys, a PAT, GitHub OAuth app registration or an Outpost account. OP-002/003 task snapshots include this latest integration requirement. The original acceptance contracts below remain reference snapshots; GitHub Issues own current task requirements.

## Published issue map

| Planning ID | GitHub Issue |
| --- | --- |
| OP-001 | [#1](https://github.com/felixlheureux/outpost/issues/1) |
| OP-002 | [#2](https://github.com/felixlheureux/outpost/issues/2) |
| OP-003 | [#3](https://github.com/felixlheureux/outpost/issues/3) |
| OP-004 | [#4](https://github.com/felixlheureux/outpost/issues/4) |
| OP-005 | [#5](https://github.com/felixlheureux/outpost/issues/5) |
| OP-006 | [#6](https://github.com/felixlheureux/outpost/issues/6) |
| OP-007 | [#7](https://github.com/felixlheureux/outpost/issues/7) |
| OP-008 | [#8](https://github.com/felixlheureux/outpost/issues/8) |
| OP-009 | [#9](https://github.com/felixlheureux/outpost/issues/9) |
| OP-010 | [#10](https://github.com/felixlheureux/outpost/issues/10) |
| OP-011 | [#11](https://github.com/felixlheureux/outpost/issues/11) |
| OP-012 | [#12](https://github.com/felixlheureux/outpost/issues/12) |
| OP-013 | [#13](https://github.com/felixlheureux/outpost/issues/13) |
| OP-014 | [#14](https://github.com/felixlheureux/outpost/issues/14) |
| OP-015 | [#15](https://github.com/felixlheureux/outpost/issues/15) |
| OP-016 | [#16](https://github.com/felixlheureux/outpost/issues/16) |

## Execution order

| Order | Task | Dependencies | Result |
| --- | --- | --- | --- |
| 1 | OP-001 | None | Runnable Go executable with embedded React UI and local persistence |
| 2 | OP-002 | OP-001 | Private Tailscale access and independent supervised execution |
| 3 | OP-003 | OP-001, OP-002 | URL onboarding, managed clones, readiness, and project switching |
| 4 | OP-004 | OP-003 | GitHub-backed issue specifications and durable source operations |
| 5 | OP-005 | OP-004 | One shared planning-branch queue with conflict handling |
| 6 | OP-006 | OP-003, OP-004, OP-005 | Usable Inbox, Queue, task inspector, and start panel |
| 7 | OP-007 | OP-002, OP-003, OP-005 | Durable admission, Follow queue, and workspace exclusion |
| 8 | OP-008 | OP-007 | Real Codex App Server execution with recorded capabilities |
| 9 | OP-009 | OP-007, OP-008 | Bounded activity, SSE reconnect, and private artifacts |
| 10 | OP-010 | OP-006, OP-008, OP-009 | Work view, live direction, stop, and explicit continuation |
| 11 | OP-011 | OP-008, OP-009, OP-010 | Runner-owned checks, reusable evidence, and bounded repair |
| 12 | OP-012 | OP-004, OP-005, OP-011 | Verified Git delivery and recoverable GitHub reporting |
| 13 | OP-013 | OP-009, OP-010, OP-011, OP-012 | History, actionable recovery, and useful diagnostics |
| 14 | OP-014 | OP-006, OP-010, OP-013 | Usable phone/tablet operations and accessible UI |
| 15 | OP-015 | OP-004, OP-005, OP-007, OP-012 | Previewed legacy import and safe cutover mapping |
| 16 | OP-016 | OP-001 through OP-015 | Packaged release, backup/update path, and real owner acceptance |

This order keeps an actual UI available early. Do not replace functional integration with a permanent simulated dashboard while implementing later tasks. Fixture providers may be used explicitly for tests/development and must be labeled.

## Shared implementation and verification rules

- GitHub Issues own requirements. The single planning manifest owns order. SQLite owns execution, cached observations, and evidence. Do not introduce a second task or queue authority.
- Build Go + Echo and React/TypeScript/Vite/Tailwind/shadcn. Node/Vite are build/development tools; the release UI is served from the Go executable.
- Default to one active task globally, one dedicated checkout per project, and no automatic cleanup of unfinished changes.
- Follow queue refreshes between tasks. An active assignment remains pinned; pending tasks may change. Manifest order is the scheduler order after eligibility checks.
- Use the prototype for density, navigation, contextual controls, and flow. It is illustrative state, not a backend implementation, API contract, or exact pixel mandate. Production behavior follows the specification.
- Every remote/process effect has a durable intent/receipt and a truthful unknown state. Avoid a universal workflow framework; narrow helpers and direct services are enough.
- Run focused checks for the changed behavior. Format/generate before final acceptance. Reuse applicable receipts; metadata-only closeout does not justify another full suite.
- Each task ends with: implemented behavior; touched contracts; checks/evidence; remaining limitations; and the next task's usable integration point. Never mark real-account/systemd/Tailscale acceptance passed solely because a fixture passed.

## OP-001 — Bootstrap the standalone executable and embedded SPA

**Dependencies:** None.  
**Specification:** §§5–7, 18–19, 24–25, 30.  
**Acceptance owners:** AC-05–06, foundation for AC-37, AC-61, AC-65.

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

## OP-002 — Install Outpost and prerequisites with private access and supervision

**Dependencies:** #1 (OP-001)  
**Specification:** §§6, 12, 15, 22–23, 25.  
**Acceptance owners:** AC-01–04, AC-12–17, AC-25–26 at supervision level; AC-61–64 and AC-66.

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

## OP-003 — Add repositories with managed cloning and build the project shell

**Dependencies:** #1 (OP-001), #2 (OP-002)  
**Specification:** §§8.1, 9, 18–19, 21.1, 22, 25.  
**Acceptance owners:** AC-11, AC-13, AC-53–54, AC-57–58, AC-60; shell foundation for AC-55–56.

### Outcome

On an installation with no projects, the owner chooses Add repository, pastes a GitHub repository URL, and Outpost creates a managed checkout on its Linux host. Setup progress and any real blocker are visible; completed setup opens that project's Queue. From the first project's menu the owner can add another repository, switch projects, and retain isolated settings/history without another permanent runner.

### Implementation requirements

1. Make managed cloning the default. Normal setup asks for a GitHub repository link, not a filesystem path or an SSH preparation step. Advanced Use an existing checkout remains available and inspects its existing Git state without cloning/resetting it.
2. Normalize supported GitHub repository URLs and resolve stable repository identity before choosing a private host destination. Accept normal HTTPS repository links, including an optional `.git` suffix; reject ambiguous/unsupported hosts, unsafe protocols, embedded credentials, and paths that are not repositories with a readable correction. Do not interpolate a URL into a shell command or use its path as a trusted local path.
3. Use the installation's configured host GitHub access for private repositories, with public cloning when credentials are unnecessary. Missing access is an actionable setup state; do not create another OAuth/login flow per project. Keep credentials out of argv, URLs, progress, and logs. Source/provider readiness is reported separately.
4. Persist project/setup-operation identity and intent before cloning. Clone using a controlled argv into operation-owned staging below the installation's managed workspace root, then publish/register the completed checkout atomically. Store canonical Git/workspace identity and a managed-versus-existing ownership flag. Subsequent visits reuse the checkout.
5. Setup is an observable operation: inspecting, cloning, configuring, ready, needs_attention, or interrupted. Expose bounded actual progress, stable operation ID, and retry/cancel/reconcile behavior. Browser/web interruption must not produce a false ready project or duplicate clone. Inspect owned work before retrying; never delete/reset a registered or operator-owned checkout as cleanup.
6. Add duplicate-identity guards for repeated URLs, `.git`/case aliases, double submission, and requests from two tabs. Reopen/link the existing project or setup operation instead of making another clone. Detect canonical path aliases in advanced mode. A return visit never reclones a working project.
7. Read repository instructions and manifests, detect the default branch and stack, and inherit installation defaults. Suggest check/delivery settings with provenance when uncertain; do not run discovered dependency/build scripts just because a repository was added. Existing recorded Outpost settings and instructions are preserved. No file is required on the target repository's main branch for registration.
8. Hand source initialization to OP-004/005: connect Issues, reuse the existing planning branch/manifest, or initialize a missing one as a phase of the explicit Add repository action. Keep a successful clone when source setup fails and retry only that phase. At OP-003's intermediate milestone, source capabilities not implemented yet are explicitly pending; OP-006 proves the complete integrated flow after OP-004/005.
9. Build a true zero-project screen with a primary Add repository action. Once at least one project exists, provide a current-project menu containing Add repository even when only one project exists. List other projects when present; preserve route/deep-link intent and per-project selection, history, settings, and drafts.
10. Keep Queue/Work/History compact. A persistent global run bar names the executing project/task while another project is viewed. A selection change never stops/starts work, changes a turn target, carries another project's draft, or rewrites a running assignment. Starting another coding project requires the global execution slot to be safely released.
11. Repository setup and browsing do not create additional permanent coding runners. Bounded setup work in a new destination can coexist with one coding run elsewhere; it may not touch that run's checkout or bypass workspace exclusion. Adding a repo authorizes setup; an explicit Start authorizes coding.
12. Provide readiness for source access/setup, provider authentication, check policy, missing initial branch, disk/path errors, dirty/unassigned work in advanced mode, changed dependency baseline, and surviving processes. Keep project setup state distinct from run phase/liveness.

### Failure and edge cases

Cover inaccessible/private/renamed repositories, invalid URL, empty repository/no initial branch, disk exhaustion, network interruption, duplicate clicks, browser/web restart during clone, a completed clone with failed queue setup, and advanced mode pointing to another registered checkout. Preserve existing queues, local edits, instructions, and task assignments. A partial clone must not become ready or trigger a second active clone blindly.

### Acceptance criteria

- [ ] With zero projects, Add repository is the clear primary action; a GitHub URL is sufficient input for the default path.
- [ ] A successful add creates the checkout on the Outpost host, displays real setup stages, and opens that project's Queue after integrated source setup.
- [ ] Private access failures explain the missing host access without leaking credentials; retry preserves completed setup phases.
- [ ] Repeated URL/double-submit/reconnect returns the same project/operation; returning later reuses the existing checkout.
- [ ] Clone/source failure and interruption leave a recoverable state without overwriting registered/user-owned files or existing planning data.
- [ ] From one project, Add repository remains available; two repositories have distinct checkouts/settings/history.
- [ ] Switching projects while A executes leaves A's assignment/worker/composer target unchanged and visible in the global bar.
- [ ] Advanced existing-checkout mode preserves its original Git state and enforces canonical ownership.
- [ ] Setup alone starts no coding task, installs no repository dependencies, and adds no permanent runner.

### Verification and handoff

Use disposable local Git clone fixtures behind a narrow repository-source seam to verify staging, identity, retries, and interruption without a model. Use one integrated browser journey starting empty and adding/switching two fixture-backed repositories after OP-004/005. Separately label actual public/private GitHub account acceptance. Handoff includes setup-operation/project/readiness DTOs, retry semantics, managed ownership, source-phase integration, and project/current-run selection rules.

## OP-004 — Make GitHub Issues the task source

**Dependencies:** #3 (OP-003)  
**Specification:** §§7–10, 18–19, 23, 26.1/3/8/10.  
**Acceptance owners:** AC-07–09, AC-39, AC-45, AC-51–54, AC-58–59.

### Outcome

Issues created in a separate GitHub review session become readable Outpost tasks, and Outpost can create/edit specifications without introducing another authoritative task database.

### Implementation requirements

1. Add a narrow host-side GitHub client for repository identity, issue reads/listing, creation, updates, and later manifest/report operations. Handle pagination, bounded concurrency, request bounds, conditional reads where supported, rate limits, cancellation, and retry/backoff for safe reads.
2. Resolve a configured private host credential reference. Never return/log the credential or place it in source artifacts. Expose read/write capability failures as source diagnostics rather than generic task failure.
3. Persist stable repository/issue identity, issue URL/number, source state, content digest, fetched time, local projection revision, and parsed requirements. Source cache and immutable attempt snapshots are different owners.
4. Implement the readable issue-body contract in spec §10.2. Ordinary title/body issues are executable objectives under project defaults. Parse optional settings with bounded maintained YAML/schema validation; reject duplicate blocks, cycles, self-dependencies, invalid same-project references, and unknown/unsafe execution policy.
5. Source-provided check names resolve only to operator-configured profiles. Issue text cannot install shell checks, change credentials/access policy, or become authenticated live direction.
6. Create/edit issues through durable narrow operation receipts. Add stable operation markers where needed to reconcile uncertain creation. Repeated local request keys return the same operation. Unknown effect triggers remote inspection before replay, not an automatic duplicate issue.
7. Edit forms carry expected source digest and local revision. Re-read/compare before patch, retain drafts on detected conflict, re-read after writing, and surface concurrent-write uncertainty. Do not claim issue PATCH has the manifest SHA's atomic compare-and-swap semantics.
8. Treat externally closed issues as external/unverified state. Source edits do not rerun locally settled tasks. Preserve source revisions used by execution and explicit reopen-generation behavior.

### Managed repository onboarding

Reuse the repository identity and private host credential reference established by Add repository. After cloning, the setup coordinator probes issue/source access and connects the selected repository without another per-project sign-in ceremony. Public clone success does not imply issue/queue write permission. A source-auth failure retains the managed clone and project identity and offers retry of the failed source phase. Parsing/synchronization must not modify repository instructions or main-branch files.

### Failure and edge cases

An inaccessible issue is not automatically missing/deleted. Authentication/rate limits remain distinguishable. No webhook, public callback, or direct ChatGPT-to-tailnet connection is required. A source refresh never replaces the active assignment.

### Acceptance criteria

- [ ] A separately created ordinary GitHub Issue appears in the local source projection after sync.
- [ ] Template/default metadata parsing produces a stable task objective and effective policy.
- [ ] Invalid/cyclic metadata blocks admission with the exact issue and reason.
- [ ] Create/edit success and known/unknown failure have inspectable receipts.
- [ ] Detected edit conflicts preserve the local draft; no atomic issue-update guarantee is fabricated.
- [ ] Closed/edited issues do not fabricate checks or silently rerun completed work.

- [ ] Add repository can retain a successful clone while source access is blocked, then finish setup after access is corrected without recloning.
- [ ] Public/private repository setup uses the configured host access and never exposes credentials in URLs, progress, argv, or diagnostics.

### Verification and handoff

Use GitHub fixtures for pagination, auth/rate failures, malformed bodies, creation timeouts, and conflicts; one real configured-repository read/create/edit journey is separate acceptance. Handoff provides source DTOs/digests, parsing contract, operation receipt API, and reconciliation hooks.

## OP-005 — Implement the shared planning-branch queue

**Dependencies:** #4 (OP-004)  
**Specification:** §§8, 18–19, 26.1–6.  
**Acceptance owners:** AC-40–41, AC-44–45, AC-52–53, AC-58–59.

### Outcome

All planners and Outpost use one ordered issue-number list. Reordering changes one file and leaves issue bodies and the coding checkout alone.

### Implementation requirements

1. Read/write `queue.json` on configured `outpost/queue` through GitHub APIs. Validate `{"version":1,"issues":[...]}` with distinct positive issue numbers, bounded size/count, supported version, and valid references. Do not accept a partial queue from a malformed document.
2. Keep planning branch operations outside the execution checkout. No pull, branch switch, merge, or task-closeout queue rewrite is allowed. Missing branch/file gets a clear setup action using a known base ref; never reset an existing branch.
3. Cache manifest blob SHA, ordered identities, fetched time, and health. Resolve issue references with bounded reads and preserve source links. Do not treat private/inaccessible responses as proof an issue was deleted.
4. Expose read/sync/replace and append/remove/move operations through the canonical queue service. Require expected manifest SHA for writes. A stale SHA returns conflict with current order and preserved intended edit.
5. Reapply a move only after an explicit fresh read using current neighbors/order. Never retry the stale complete list over another planner's changes, maintain per-issue numeric ranks, or rewrite every issue.
6. Track operation IDs and reconcile uncertain writes against current blob/payload/commit observations. Keep success, rejection, and unknown separate.
7. Creation-plus-queueing is a two-step flow: if issue creation succeeds but append fails, expose the created issue in Inbox and retry append only. An issue existing in the list is a no-op append, not a duplicate entry.
8. Define source unavailability and manual Sync behavior. Background refresh is bounded/configurable; a worker always refreshes at admission independently of the UI poll.

### Managed repository onboarding

Initialize the source as a phase of the explicitly requested Add repository operation. Reuse an existing valid planning branch/file without resetting or replacing its contents. When missing and permissions/base commit allow, create `outpost/queue` and its empty root `queue.json` through GitHub APIs automatically, then mark that setup phase complete. Do not require the normal user to run the developer setup helper, edit JSON, or switch the checkout. Existing malformed/inaccessible data is a named blocker, not permission to overwrite. If source initialization fails after cloning, retry the source phase using the same project/operation. Keep main-branch files unchanged.

### Failure and edge cases

Completed/closed entries may remain in the list and are skipped with provenance; removal is an explicit planning action. Removing an active issue does not interrupt its pinned assignment. Shared queue storage does not create distributed execution locking across separate installations.

### Acceptance criteria

- [ ] One reorder creates one manifest change and no issue edits or checkout changes.
- [ ] Two stale clients cannot silently overwrite each other; conflict preserves intended order.
- [ ] Invalid versions, duplicates, missing/inaccessible references, and source outages are explained.
- [ ] An uncertain write is inspected before a retry.
- [ ] Partial creation/append failure queues the same created issue on recovery.
- [ ] The current manifest SHA and source-health state are available to scheduler/UI.

- [ ] Adding a new project initializes a missing planning branch/empty manifest automatically from the explicit setup request.
- [ ] Adding a project with an existing queue preserves its exact contents; a failed queue phase can resume without another clone.
- [ ] Neither setup nor retries modify target main-branch instructions/templates or switch the coding checkout.

### Verification and handoff

Use a focused fake GitHub contents service for SHA races/uncertainty and one real planning-branch read/write acceptance. Record checkout HEAD/status before and after to prove queue operations did not change it. Handoff documents manifest schema, error/receipt DTOs, and atomicity limits.

## OP-006 — Build Inbox, Queue, the task inspector, and start panel

**Dependencies:** #3 (OP-003), #4 (OP-004), #5 (OP-005)  
**Specification:** §21.1–6 and §§8, 10, 19.  
**Acceptance owners:** AC-07–08, AC-39–41, AC-45, AC-47, AC-53, AC-55–60.

### Outcome

The owner can capture an issue, refine it, deliberately queue it, reorder pending work, and understand what would run next without leaving a compact workspace.

### Implementation requirements

1. Follow the prototype's density, restrained controls, three-view navigation, and selected task inspector. Use actual source data and honest loading/empty/error states.
2. Implement Queued/Inbox tabs. Inbox means open unqueued issues, not a GitHub draft feature. Keep task selection and list position through refresh, edits, tab changes, and opening/closing details.
3. Show position, issue number/title, readiness, dependency/hold reason, and source link. The next eligible task may differ from row one; expose both clearly. Do not add a priority sort that competes with manifest order.
4. The inspector provides outcome/criteria, dependencies, effective checks/delivery, historical link, Edit, Queue/remove/hold, and accessible Move earlier/later actions. Do not require drag-and-drop.
5. Use one compact new/edit form. Title/outcome/criteria are primary; metadata/policy overrides are optional details. Save defaults to Inbox; Add to the end of the queue is explicit. Active specification edits clearly apply to a future attempt and link to Work for live direction.
6. Show saving/saved/conflict/unavailable and partial issue-created-but-not-queued states. Retain form text and intended order on failure. Disable/reconcile conflicting controls during unresolved operations without losing the user's input.
7. Implement a start panel showing project/branch, next eligible task/blockers, Follow queue default, optional Fixed scope, model/reasoning/actual permissions, delivery, task limit, optional runtime budget, and instructions. Until execution integration exists, clearly identify the missing capability rather than simulate success.
8. Source details and settings live behind secondary actions. Last successful sync and actionable failures are visible without exposing host credentials or making ordinary users manage JSON.

### Managed repository onboarding

Connect the Add repository backend from OP-003 with the source phases from OP-004/005. The first-run screen has no fake selected project: it offers Add repository, a GitHub URL field, and an Add action. Show actual inspection/clone/configuration progress and the failed stage with an appropriate retry action. On successful setup, open the new project's Queue. Once one project exists, its project menu still contains Add repository; with two projects it also switches between them. Project selection, drafts, queues, settings, and history remain isolated. Setup progress survives browser reconnect and duplicate submission without creating a second clone. While A executes, adding/viewing B keeps A visible in the global run bar; B's Start explains the occupied execution slot rather than stopping A or queueing an automatic cross-project handoff.

### Failure and edge cases

Selection survives a source reorder; removal/closure of the selected issue has an explicit state. Two-tab conflicts keep both source and local intent inspectable. A failed append must not invite another Create issue click as the only recovery path.

### Acceptance criteria

- [ ] Create → Inbox → inspect/edit → queue → move works against the backend contracts.
- [ ] Inspector navigation preserves list context and unsaved form text.
- [ ] Queue position and eligibility are distinguishable, including dependencies and holds.
- [ ] Stale SHA/source edit conflicts preserve drafts and never claim a rejected write was saved.
- [ ] The start panel makes its scope and effective settings clear without a second mandatory approval screen.
- [ ] Keyboard and basic narrow-screen operations work before the later responsive pass.

- [ ] From an empty installation, paste a repository URL → see setup progress → land in the project's Queue without supplying a local path.
- [ ] Add a second repository from the project menu, switch between A/B, and revisit each without recloning or mixing state.
- [ ] While A executes, add/view B and return to A; the global run bar and unsent instructions remain correctly targeted.
- [ ] Clone/source failures show the actual failed phase and retry it without losing a completed checkout or an existing queue.

### Verification and handoff

One browser journey covers creation, partial failure, reorder conflict, and task selection; avoid component snapshots mirroring markup. Handoff identifies the real start request payload and the selected issue/source revision used by the scheduler.

## OP-007 — Add durable scheduling, Follow queue, and task admission

**Dependencies:** #2 (OP-002), #3 (OP-003), #5 (OP-005)  
**Specification:** §§9–12, 18–19, 26.  
**Acceptance owners:** AC-09–16, AC-25, AC-40–44, AC-51, AC-55–58.

### Outcome

A run admits exactly one owned task at a time, follows fresh queue changes between tasks, and preserves the active assignment across planning edits and failures.

### Implementation requirements

1. Define run/attempt/execution-generation state, captured settings/budget/mode, immutable assignment snapshot, and the reservation/lock protocol shared with systemd. Use explicit expected-revision guards and unique constraints.
2. Default Follow queue to fresh manifest and relevant issue/dependency reads before each admission. Cache failures cannot authorize new work. The assigned task keeps its issue-body digest, manifest SHA/position, workspace baseline, instructions, and policy.
3. Walk manifest order after excluding settled generations, closures, holds, unavailable dependencies, and already-owned tasks. Do not sort by priority. Validate dependency graph and usable predecessor outputs, not just a done label.
4. Add optional Fixed scope as a captured ordered identity set; later additions stay out. Still refresh current issue state/definitions and prerequisites. Manual single-task and instruction runs remain explicit kinds and do not accidentally advance the queue.
5. Source requests happen outside DB transactions. After fetching, atomically compare local control/source observations and claim one assignment. Define the observation/assignment cutover honestly; remote writes after observation apply at the next boundary.
6. Check stop and budget at the same guarded admission boundary. New queue additions can join within the remaining budget; removals/reorder affect pending work only. Record the reason for skips and selected task.
7. Count successful tasks only after checks/delivery settle. Failure/blocked/interrupted work stops automatic advancement and keeps unfinished edits assigned. An empty eligible set distinguishes drained, budget reached, externally closed, and truly blocked work.
8. Reconcile surviving work before another assignment. A completed task remaining in the manifest is skipped even if GitHub reporting is pending. Source body changes do not implicitly reopen a local execution generation.

### Managed repository onboarding

Project browsing and repository setup are independent of the single global coding slot. A run remains scoped to its recorded project and never switches when the user selects another repository. Do not automatically start B after A drains. After an explicit stop/interruption of A, release the global slot only when all owned coding/check processes are settled and uncertain effects reconciled; unfinished A remains assigned to its own checkout/history. B may then start in its separate workspace without discarding or repairing A first. Resuming A later is explicit and rechecks its preserved assignment/baseline. If A still owns a child/process, explain why B cannot start.

### Failure and edge cases

External issue closure is not check evidence. A changed checkout may invalidate predecessor availability. Queue outage lets active work finish but blocks the next admission. A drained run settles rather than polling forever. A run reservation is not released just because a display status looks terminal.

### Acceptance criteria

- [ ] With `[72,41,88]` and `72→41`, the first eligible task is 41 and explanations remain visible.
- [ ] A newly queued task runs at the next eligible boundary within budget, while the active task stays pinned.
- [ ] Fixed scope excludes additions and both modes honor source outages/stop/baseline checks.
- [ ] Concurrent starts/control races produce one writer and no unwanted next task.
- [ ] Failed assigned work is preserved and not abandoned for another task.
- [ ] Completion/closure/reopen behavior preserves execution generations and provenance.

- [ ] Selecting/adding another project never retargets a running assignment or starts a second coding worker.
- [ ] After safe settlement of A, B can run while A's preserved unfinished checkout remains recoverable; surviving A processes still prevent a conflicting start.
- [ ] A drained run does not automatically jump to another project's queue.

### Verification and handoff

Use real SQLite concurrency, temporary Git baselines, and a deterministic fixture worker. Exercise source-change/admission timing and stop-after-task races. Handoff includes the assignment DTO, transition guards, next-eligible projection, and provider lifecycle integration point.

## OP-008 — Integrate the real Codex App Server adapter

**Dependencies:** #7 (OP-007)  
**Specification:** §§11–14, 23, 28.  
**Acceptance owners:** AC-16, AC-26–31 at adapter level, real-provider AC-38, AC-63, AC-66.

### Outcome

An owned task can run through installed Codex using its actual supported App Server stdio protocol, with real capabilities/settings/session identity and structured results.

### Implementation requirements

1. Inspect the installed/supported Codex version and current official protocol schemas. Pin/document compatibility. Reuse useful behavior from `scripts/runner_codex.py` without copying Python transport assumptions blindly.
2. Implement a narrow adapter boundary for capabilities, session start/resume, turn start, steering, interruption, events, and structured completion. Keep provider-specific IDs/errors inside this adapter and durable receipts.
3. Discover supported model/reasoning/permission choices. Record actual selected model, reasoning, execution permissions, provider version, session ID, and turn ID. Unsupported capabilities remain explicit; no invented defaults or silent emulation.
4. Start the owned provider with bounded stdio handling, stderr capture, cancellation, frame/message limits, and well-defined process settlement. Do not buffer all output in memory.
5. Compose immutable effective prompts from runner contract, repository/project instructions, pinned issue snapshot, labeled workspace evidence, and authenticated operator direction. Preserve trust boundaries; source/log content is not operator control.
6. Parse structured handoff fields into proposed results, changed paths, claimed checks, blockers, and summary. Provider completion and claimed checks remain unverified until the runner's own policy settles.
7. Resume an explicit saved session when identity/capability/workspace agree. A missing or rejected session is a clear failure; fresh conversation requires explicit selection and a new linked attempt.
8. Normalize provider failures into actionable authentication, protocol, permission, interruption, and unknown-outcome states. Provider permission/OS boundaries cannot be elevated by prompt text.

### Installation and first-run contract

Supply the installer with a verified official Codex distribution/version and its actual runtime prerequisites. Detect an existing installation/account before changing anything. Document authentication/status diagnostics and compatibility failures in INSTALL.md. If the chosen distribution needs Node or another runtime, make that an explicit provider prerequisite; do not introduce a frontend dev server or imply the embedded dashboard itself requires Node.

### Failure and edge cases

Malformed/oversized frames, stderr spam, lost terminal acknowledgement, missing executable, authentication expiry, and unexpected process exit all preserve task assignment and receipts. Avoid automatically retrying a turn whose effects are unknown.

### Acceptance criteria

- [ ] Fixture protocol sessions prove start/events/result/error/cancellation behavior and resource bounds.
- [ ] Actual discovered settings are visible and stored with the attempt.
- [ ] Saved-session failure never silently starts a fresh conversation.
- [ ] A real authenticated Codex task performs a small disposable repository change with recorded identities.
- [ ] Agent claims are not marked runner-verified checks or delivery.

- [ ] The installer/doctor can identify the actual supported Codex version, distribution requirements, and authentication state.
- [ ] A missing/unsupported provider installation or sign-in produces actionable first-run guidance rather than simulated readiness.

### Verification and handoff

Keep local protocol tests independent of paid-model credentials. Separately record the real version/account smoke result. Handoff supplies capabilities, normalized events, task handoff schema, steering/interrupt calls, and actual session-recovery rules for Work/controls.

## OP-009 — Deliver bounded live activity, reconnect, and private artifacts

**Dependencies:** OP-007, OP-008.  
**Specification:** §§18–20, 23, 28.  
**Acceptance owners:** AC-15, AC-32–35, foundations for AC-48.

### Outcome

The browser can observe actual execution, reconnect after a network gap, and inspect retained output without unbounded memory use, lost drafts, or unsafe artifact paths.

### Implementation requirements

1. Persist bounded structured activity/events with cursors and private artifact references. Define normalized task/phase/message/check/delivery/source-health events; business-state receipts remain authoritative.
2. Serve an initial consistent projection/cursor, replay newer events, then follow SSE. Support Last-Event-ID and explicit reset when retention removes the required range. Restarted web service reconstructs state from storage and owned-process observations.
3. Bound subscriber queues and disconnect/reset slow clients. Backpressure must not block provider/check execution or accumulate unlimited process output.
4. Store full allowed output under private run/attempt artifact paths. Expose bounded previews, truthful truncation, explicit downloads, pagination/tail bounds, and size/disk diagnostics. Active/unresolved artifacts are never removed by retention actions.
5. Resolve downloads from allowlisted IDs/manifests; reject traversal, symlink escape, directories, and nonregular files. Enforce the same private identity boundary as API/SSE.
6. Escape logs and sanitize supported Markdown; provider output cannot inject active HTML/script. Keep private environment values/tokens out of ordinary errors.
7. Update only affected frontend resources. Preserve composer text/focus, selection, expanded message readers, and reading position. Following live updates is explicit; reading earlier output does not jump to the bottom on each event.
8. Expose connection freshness and last meaningful event separately from process liveness. A network gap must not show a false stopped or live state.

### Failure and edge cases

Exercise large command output, incomplete Unicode/lines, reconnect after cursor expiry, a slow phone connection, simultaneous viewers, and artifact disappearance after a valid settled retention action. A missing preview cannot silently pretend the complete output was displayed.

### Acceptance criteria

- [ ] Web restart and SSE reconnect return actual run state with correct cursor/reset behavior.
- [ ] Large output/slow subscribers remain bounded without blocking execution.
- [ ] Logs remain inert; forbidden artifact paths and denied identities are rejected.
- [ ] Reading older messages and typing instructions survive new activity.
- [ ] Last-seen/connection state does not claim a process stopped merely because SSE disconnected.

### Verification and handoff

Use bounded stress fixtures and focused replay/path tests plus one reconnect browser journey. Handoff documents event schemas, replay bounds, artifact API, frontend invalidation strategy, and notification retention policy.

## OP-010 — Implement Work, live instructions, stop, and continuation

**Dependencies:** OP-006, OP-008, OP-009.  
**Specification:** §§14–15, 19, 21.2/7–9.  
**Acceptance owners:** AC-16, AC-25–31, AC-48–49.

### Outcome

The owner can monitor and steer real work, request a clean stop after the task, interrupt immediately, and continue preserved work without losing typed instructions or targeting the wrong turn.

### Implementation requirements

1. Build Work around the prototype: task/settings/phase header, Activity/Changes/Checks tabs, optional wide context panel, and one stable composer. Show actual model/reasoning/permissions and completed count/budget. No invented percentage.
2. Activity uses bounded normalized events; Changes reads actual changed paths and lazy diffs; Checks integrates receipts as OP-011 lands. Keep task brief available when the context panel collapses.
3. Persist live message intent with client ID, payload digest, attempt/session/expected-turn target, and queued/sending/accepted/rejected/unknown states. Accepted means provider acknowledgement, not implementation success.
4. Reject stale targets without silently redirecting to another turn. Between managed turns, allow explicit next-turn delivery. Lost acknowledgement becomes unknown; do not automatically resend. Duplicate key/payload returns its receipt; changed payload conflicts.
5. Preserve the same composer through routes, events, rejection, interruption, and reload using scoped session storage. Retain text until delivery is acknowledged/reconciled, and keep unknown submissions inspectable. Never carry a draft into another project's active task automatically.
6. Implement durable Finish task & stop, Undo stop before next admission, and Interrupt now. Keep stop state globally visible; interruption observes provider and owned-child settlement and does not merely set a local UI flag.
7. After interruption/blocking, offer Continue with instructions, saved-session resume when available, and explicit fresh conversation. Create linked attempt history and retain budget/count. State whether queue processing resumes afterward.
8. Active specification edits remain future task revisions; live steering records current operator instructions and can change the validation contract. Keep old source/prompt evidence immutable.

### Failure and edge cases

Cover message dispatch racing turn completion, stop racing next assignment, continuation while an owned child survives, missing saved session, repeated button presses, and browser reconnection after an unknown submission. Avoid two composers or controls on an old historical attempt.

### Acceptance criteria

- [ ] Actual turn-directed instructions have visible durable delivery state.
- [ ] Stale/unknown/duplicate messages behave truthfully and preserve user input.
- [ ] Finish-after-task prevents the next guarded admission; Undo cancels only while applicable.
- [ ] Interrupt preserves diff/session/assignment and waits for real settlement.
- [ ] Saved/fresh continuation is explicit and retains remaining budget/history.
- [ ] Queue/Work/History navigation and SSE never discard unsent instructions or move a reader unexpectedly.

### Verification and handoff

Use targeted protocol/control race tests and a browser journey typing before navigation, stale-target rejection, interruption, and continuation. Add a real provider steering/interrupt acceptance separately. Handoff includes message/stop/continuation receipts and the contract-change signal for validation.

## OP-011 — Own validation receipts and bounded repair

**Dependencies:** OP-008, OP-009, OP-010.  
**Specification:** §16 and §§9, 12, 18, 21.7, 29.  
**Acceptance owners:** AC-17–23.

### Outcome

Outpost can prove which required checks ran against which inputs, reuse valid evidence, repair actual failures within a bound, and recover owned check results without repeating work unnecessarily.

### Implementation requirements

1. Resolve check plans from operator-configured profiles and accepted task criteria. Distinguish required/optional checks and runner-owned results from agent claims. Imported/issue text can select approved profiles, not invent executable commands.
2. Run argv-based owned check processes with defined working directory/environment policy, process identity, timeout/cancellation, bounded output, and durable terminal receipts. Keep workspace exclusion while any owned check can still write.
3. Define relevant input identity using tracked/relevant-untracked code, configuration, fixtures, tool/check contract, and declared supplemental inputs. Keep title, queue position, status, completion notes, and UI metadata out of a code check's identity unless a real check explicitly consumes them.
4. Format/generate before final acceptance. A no-op formatter or metadata-only closeout must not invalidate otherwise applicable receipts. Actual relevant code/config/criterion changes invalidate affected checks. Never reuse solely because a task ID matches.
5. Store command snapshot, generation, input/contract digest, start/end/duration, process identity, exit/signal, case-count requirements, status, and artifact links. A configured nonzero-case check cannot pass on zero tests.
6. On restart, reconcile the exact child or terminal receipt before rerun. Unknown process completion stays unknown. A surviving validation child blocks another writer and has an actionable stop/inspect path.
7. Send bounded real failure evidence back to the same assignment for repair. Default repair limit is three cycles; unchanged repeated failure/exhaustion stops with evidence. Missing credentials/external services remain explicit blockers rather than fake passes.
8. Integrate Checks UI: required/optional, passed/reused/failed/interrupted/not-run, duration, relevant identity, reason for reuse/invalidation, and retained output. Local success cannot settle until all required policy is met.

### Failure and edge cases

Cover supervisor death after check completion, child survival, output overflow, timeout/signal, zero collected tests, meaningful operator contract changes, and a no-op formatter. A terminal agent response does not settle a check process.

### Acceptance criteria

- [ ] Real check failure reaches bounded repair and cannot be reported as a pass by prose.
- [ ] A valid terminal receipt survives supervisor interruption and is reused after reconciliation.
- [ ] Metadata edits/no-op formatting do not trigger an unchanged full gate.
- [ ] Relevant code/config/contract changes reject stale evidence.
- [ ] Zero-case requirements, surviving children, and exhausted repair have clear outcomes.
- [ ] The UI explains what ran, what was reused, and why.

### Verification and handoff

Use small executable fixtures and disposable real Git inputs for receipt identity/recovery; avoid slow monorepo gates for fixture behavior. Handoff defines final acceptance contract, evidence generation, repair state, and the verified candidate identity used by Git delivery.

## OP-012 — Verify Git delivery and report results to GitHub

**Dependencies:** OP-004, OP-005, OP-011.  
**Specification:** §§16.9–17, 18, 26.7–9.  
**Acceptance owners:** AC-21, AC-24, AC-46, AC-51–52.

### Outcome

Completion means the configured local/commit/push policy actually settled. A later GitHub reporting outage cannot cause repeated coding work or damage the shared queue.

### Implementation requirements

1. Support project-selected local_changes, commit, and push outcomes. Optional review is a policy, not a universal new approval flow. Record actual candidate, relevant check receipts, changed-path ownership, branch/upstream, and delivery observations.
2. Never bulk-stage unrelated files or reset/stash source to simplify delivery. Detect ambiguous pre-existing changes/commits and provide explicit recovery/investigation rather than guessing ownership.
3. Commit only the accepted candidate and intended owned files. Verify resulting commit and intended remote/branch before considering push settled. A post-check meaningful code change requires relevant revalidation; allowed metadata closeout does not repeat an unchanged gate.
4. Reconcile uncertain commit/push outcomes through Git facts and remote containment. Do not create another commit or blindly repeat effects merely because an acknowledgement was lost.
5. Settle local task/attempt/completed count only when required policy is met. Preserve immutable receipts and separate optional review outcome from provider text.
6. Persist a narrow GitHub reporting operation with stable identity/marker after local completion. Post issue comment with outcome, delivered commit/branch, check summary, limitations, and retained local run identity. Exclude tokens, host paths, full prompts, and large private logs.
7. Close the issue according to configured completion policy only. Reporting/closing failure leaves local completion settled and visibly pending; retry/reconcile that remote step without another coding attempt. Unknown comment creation checks the stable marker before replay.
8. Never remove/reorder queue entries as part of closeout. The scheduler skips settled generations. External closure does not fabricate local evidence; reopening execution is explicit.

### Failure and edge cases

Handle detached/no-upstream configuration, remote rejection, authentication loss, commit created before crash, push succeeded before status write, report succeeded before acknowledgement, and issue already closed. Preserve uncertainty without deadlocking unrelated settled reporting forever.

### Acceptance criteria

- [ ] Each delivery mode has a real receipt proving its configured outcome.
- [ ] Unrelated changes remain untouched and ambiguous ownership blocks safely.
- [ ] Commit/push recovery inspects existing effects rather than duplicating them.
- [ ] GitHub reporting outages remain pending while local success stays settled.
- [ ] Reporting resumes with the same operation identity and no new coding attempt.
- [ ] Completion does not rewrite the manifest; next work uses the existing order.

### Verification and handoff

Use local bare remotes for Git behavior and mocked GitHub outcomes; record a real configured push/comment/close journey separately. Handoff exposes outcome/reporting projections for History and the dependency-baseline receipt for the next task.

## OP-013 — Make history, blockers, and recovery actionable

**Dependencies:** OP-009, OP-010, OP-011, OP-012.  
**Specification:** §§8.6–7, 12, 15, 21.9–10, 22, 28.  
**Acceptance owners:** AC-14–18, AC-24, AC-31, AC-49.

### Outcome

The owner can understand an earlier attempt, find its evidence, and recover current work through ordinary UI actions without editing SQLite, deleting locks, or reading raw process logs first.

### Implementation requirements

1. History lists real runs and linked attempts, including launch failures, blocked/interrupted outcomes, continuations, settings, check/delivery summary, and pending GitHub reporting. Preserve original outcomes.
2. Historical detail is read-only and links to current work/continuation explicitly. Never let an old attempt's composer or stop button issue controls against a newer target implicitly.
3. Use one failure/attention panel near the current composer: concrete reason, known surviving work/processes, last observation, and relevant action. Authentication, failed checks, invalid queue, baseline mismatch, missing session, and surviving child require different guidance.
4. Reuse the canonical continuation, stop, source-sync, and reporting operations. Recovery must not become a second unguarded launch path or a set of arbitrary shell commands.
5. Connection diagnostics show private address, actual Tailscale/systemd observations, allowed identity state, provider/source readiness, version, state/disk use, and last useful observation. Sanitize sensitive details; expose bounded diagnostic artifacts separately when needed.
6. Provide run/attempt/turn/message/check/delivery/artifact navigation without adding more primary app sections. Attempt details expose captured issue/manifest revisions and actual model/reasoning/permissions; missing imported fields say Not recorded.
7. Manual settled-artifact retention/archive is bounded and never deletes active/unresolved evidence. Explain what remains after an artifact is removed. No automatic retention framework is required.

### Failure and edge cases

Old history remains readable when the provider session no longer exists. An unavailable host observation is unknown, not proof of no process. Recovery of reporting is separate from recovery of coding. A historical success cannot be rewritten into a different outcome by a later attempt.

### Acceptance criteria

- [ ] Interrupted/blocked/failed/startup runs and their continuations are navigable with immutable evidence.
- [ ] Historical controls cannot accidentally target current work.
- [ ] Current blockers offer appropriate ordinary recovery and retain instruction drafts.
- [ ] Source/provider/systemd uncertainty remains visible and actionable.
- [ ] Retention actions reject active/unresolved artifacts and do not expose private files.

### Verification and handoff

Use one cross-state browser journey backed by stored fixture runs and focused destructive-retention guards. Handoff documents diagnostic meanings, blocker-action mapping, and recovery links for phone acceptance.

## OP-014 — Finish phone/tablet operations and accessibility

**Dependencies:** #6 (OP-006), #10 (OP-010), #13 (OP-013)  
**Specification:** §21, especially 21.11–12; §22.  
**Acceptance owners:** AC-32, AC-47–50, AC-53, AC-55–58.

### Outcome

The same app is comfortable for quick Tailscale check-ins from a phone and does not squeeze several desktop columns into a tablet viewport.

### Implementation requirements

1. Inspect real routes at 320, 390, 768, 820, 1024, and 1440 CSS pixels. Collapse the sidebar and task/context panels before content becomes cramped. Use a full-width inspector sheet or detail route at narrow sizes, preserving the queue's selection/scroll on return.
2. Keep the current run state and finish/stop/recovery controls reachable. Overflow actions must work on touch without hover. The composer remains full-width and usable with the mobile keyboard open.
3. Verify core phone flows: current status, inspect blocker, send direction, finish-after-task/undo, interrupt, saved/fresh continuation, inspect checks, and move next work.
4. Prevent application-level horizontal overflow. Wrap issue titles, source errors, branch/commit identifiers, metadata, and form controls. Restrict horizontal scrolling to actual code/diffs/logs where it helps.
5. Use comfortable touch targets, 16px editable text where necessary, logical focus order, visible focus, native labeled controls, accessible tab/dialog behavior, Escape/close return focus, and keyboard alternatives for queue movement.
6. Check light/dark contrast and semantic statuses paired with text. Honor reduced-motion and avoid continual animation/fake progress.
7. Reconnect after sleep/network loss without discarding drafts, falsely claiming liveness, jumping the reader, duplicating a message, or reissuing an execution control automatically.
8. Remove redundant settings/navigation and prototype-only explanatory text from product flows. Keep implementation details secondary unless they explain a meaningful decision.

### Managed repository onboarding

Include zero-project onboarding and the project menu in the phone/tablet pass. The GitHub URL form and actual clone/configuration progress fit without horizontal overflow, with an actionable retry state. Add repository remains reachable with one project. With two projects, the selector can switch context while the global run bar still identifies the executing project. Long owner/repository names wrap, and a phone reconnect does not submit setup twice or lose the current operation/draft.

### Failure and edge cases

Use a long task title, long unbroken branch/path, empty Inbox, source conflict, stop-pending state, and unknown message receipt. Test a dialog opened after scrolling and a draft typed before a route change.

### Acceptance criteria

- [ ] Every target width has readable primary content without page overflow or squeezed fixed columns.
- [ ] Phone operational flows are reachable with the keyboard open and touch controls usable.
- [ ] Keyboard users can inspect/edit/move tasks, send instructions, and close dialogs with sensible focus return.
- [ ] Reconnection/navigation preserves scoped drafts and reading position.
- [ ] Light/dark/status/focus contrast passes the chosen accessibility checks.

- [ ] On phone and tablet, empty-state URL onboarding, Add another repository, switching, setup failure/retry, and reconnect are usable.
- [ ] With A running and B selected, touch controls and draft targets remain unambiguous.

### Verification and handoff

Use representative browser viewport checks and visual inspection, not a separate snapshot for every element/state. Record actual device verification separately from emulation. Handoff includes viewport evidence, any known browser limitations, and changes from the concept made for better real usability.

## OP-015 — Import legacy tasks and prepare a safe VibeEasy cutover

**Dependencies:** OP-004, OP-005, OP-007, OP-012.  
**Specification:** §§9, 16, 27.  
**Acceptance owners:** AC-09, AC-11, AC-36, AC-45, AC-51–52.

### Outcome

Existing Markdown/VibeEasy work can be previewed and mapped to GitHub Issues and the shared manifest without duplicate tasks, fabricated success, or competing runners.

### Implementation requirements

1. Inspect current VibeEasy task/queue/check conventions at implementation time. The previously reviewed `runner-dashboard/` and `scripts/runner_*.py` are behavioral references; do not assume every format stayed unchanged.
2. Implement a bounded general Markdown importer/exporter and focused VibeEasy adapter using a maintained metadata parser. Keep source identity, original number/path, requirements, dependencies, holds, and validation-profile references.
3. Preview create/update/skip actions, unresolved dependencies, duplicate/colliding IDs, unsupported policy, queue order, imported historical status, and baseline requirements before applying.
4. Apply with durable per-issue mapping/receipts, then a separate manifest update. A crash or append conflict resumes from created identities; no all-or-nothing GitHub/SQLite transaction is claimed.
5. Preserve imported historical outcomes as unverified provenance. Missing model/permission/check fields remain Not recorded. Imported done prose alone cannot establish a check receipt or predecessor code in the current checkout.
6. Export a readable snapshot including source issue identity/revision. Original task files and exports never become a second silently writable source authority.
7. Write a cutover runbook: test disposable checkout first; settle old worker/check children; inspect/preserve the chosen checkout; confirm issue/dependency/hold mappings; prevent two writers; run one small new-path task; retain the old installation for fallback until accepted.
8. If VibeEasy still requires updating original evidence files, implement only a narrow configured closeout action with defined inputs/checks. It must not restore competing queue ownership or require redundant code gates after metadata-only updates.

### Failure and edge cases

Importing again does not duplicate issues. Conflicting source IDs do not overwrite unrelated work. The old Python journal is never treated as an Outpost Goose database; history import reads a preserved/consistent copy.

### Acceptance criteria

- [ ] A representative current VibeEasy task set previews with accurate mappings/warnings.
- [ ] Apply/retry preserves created issue identities and valid dependency/order references.
- [ ] Imported completion remains distinct from runner verification and baseline evidence.
- [ ] Queue writes target the planning branch and never silently rewrite old runner sources.
- [ ] The runbook prevents concurrent ownership and preserves old source/history.

### Verification and handoff

Use a small representative fixture plus a read-only preview of actual current tasks. A real cutover is separate owner/environment acceptance, not implicit permission to alter the old installation. Handoff includes mapping/report, unsupported cases, and rollback boundaries.

## OP-016 — Package the release and prove the complete owner workflow

**Dependencies:** #1 (OP-001), #2 (OP-002), #3 (OP-003), #4 (OP-004), #5 (OP-005), #6 (OP-006), #7 (OP-007), #8 (OP-008), #9 (OP-009), #10 (OP-010), #11 (OP-011), #12 (OP-012), #13 (OP-013), #14 (OP-014), #15 (OP-015)  
**Specification:** §§24–25, 28–29, 33–34.  
**Acceptance owners:** Final integration of AC-01 through AC-66, especially AC-37–38 and AC-53–66.

### Outcome

The first release can be built, installed, backed up, updated, and used for real GitHub-backed Codex work from the owner's Mac and phone over Tailscale.

### Implementation requirements

1. Produce a documented reproducible release build with frontend assets/migrations embedded, pinned dependencies, version metadata, and platform target. Keep Git/Codex/Tailscale/systemd/repository toolchains explicit host prerequisites.
2. Document idle/maintenance backup of database and referenced artifacts/config, excluding careless token exposure. Coordinate schema migration with owned workers/checks; reject unsupported newer databases and unsafe active migration. Restore a representative backup.
3. Complete the actual sequence: inspect pushed code in a separate GitHub session; create an issue; observe Inbox; queue/reorder; start Follow queue; perform real Codex work; send direction; finish/check/deliver; observe completion reporting; inspect History from another device.
4. Demonstrate source addition during execution affecting the next boundary and pinning the active task. Demonstrate a safe stop/continuation and a source/reporting interruption without duplicate coding work.
5. Restart the web service during active work and reconnect. Separately exercise the specified process/receipt failure paths with fixtures; do not require deliberately destructive failure against valuable live work.
6. Map AC-01 through AC-66 to existing focused test/journey evidence. Combine related cases rather than creating 66 standalone tests. Identify unavailable real-account/host/device evidence honestly.
7. Run final formatting/generation before the agreed relevant gate: Go vet/tests, frontend typecheck/build, required browser journeys, sqlc freshness, and embedded-binary smoke. Use race/host checks where changed concurrency/supervision warrants them. Do not restart unchanged full gates after documentation-only evidence edits.
8. Record measured idle/resource behavior with host/workload context if available. Do not claim Go memory/speed advantages from language choice alone.
9. Finalize operator quickstart, GitHub credential/source setup, task template, queue conflict/recovery guide, checks/delivery configuration, stop semantics, logs/retention, backup/update, and troubleshooting.

### Managed repository onboarding

Begin the owner journey with zero registered projects. Add repository by GitHub URL and confirm a host-side managed checkout plus source initialization. Add a second repository through the same menu; switch queues/settings/history and revisit without recloning. Run A while browsing B, preserve drafts and the global bar, safely stop/settle A, then start B with the same single coding slot and return to A's preserved work. Verify existing planning data and target main-branch files survive setup.

### Installation and first-run contract

Ship actual release artifacts and checksums consumed by scripts/install.sh. Validate INSTALL.md from a clean supported Linux host and a separate clean source checkout: release bootstrap/local-binary install, required dependencies, one-time Tailscale/Codex/GitHub setup, unprivileged service and boot behavior, real dashboard URL, then empty-state Add repository. Re-run installation to prove preservation/idempotency; test an unavailable dependency/authentication path without false success. Keep source-only Go/Node toolchains separate from dashboard runtime and state any provider-specific requirement. Replace the starter guide's planning/status notices with actual verified release commands and compatibility before calling this release installable.

### Failure and edge cases

Fixture success cannot substitute for actual Codex/Tailscale/device acceptance. Pending reporting is a visible limitation, not an excuse to mark a fresh run active. A release with simulated activity or no real worker control is incomplete.

### Acceptance criteria

- [ ] The standalone executable serves the real application without a dashboard Node/Vite runtime.
- [ ] The owner completes a real issue → queue → Codex → direction → checks → delivery → GitHub report journey over Tailscale.
- [ ] Mac/phone revisit, web restart, stop/continuation, and pending reporting remain usable and truthful.
- [ ] Backup/restore and guarded update/migration preserve records.
- [ ] The acceptance map names evidence and any remaining limitations explicitly.
- [ ] No deferred platform features are required to use the release.

- [ ] The actual owner journey includes empty-state URL cloning, a second repository, project switching/reuse, and one safely transferred coding slot.
- [ ] A setup failure can be retried at its failed phase; existing queue/main-branch content and unfinished work remain intact.

- [ ] The shipped INSTALL.md and installer work on a clean supported host and the source-build instructions work from a clean checkout.
- [ ] Release asset/checksum selection, dependency/auth setup, idempotent reinstall, and the final private URL are demonstrated with real host evidence.
- [ ] Unsupported/unavailable prerequisite cases remain actionable and never report a ready installation.

### Verification and final handoff

Provide release/build identity, installation address, tested tool/provider versions, concise acceptance map, measured resource notes if any, known limitations, and upgrade/backup commands. Keep secrets and private logs out of public issue reports. Deferred work remains a separate backlog rather than expanding this release indefinitely.

## Prototype reference and deliberate limits

Open `Outpost-UX-Reference.html` in a browser. It is a standalone exported reference with illustrative tasks and in-memory interactions. Its Queue/Work/History navigation, task selection/movement, issue-form shape, Activity/Changes/Checks, stop/interrupt/continue, and contextual composer help explain the intended experience.

The reference does **not** call GitHub, launch Codex/systemd, perform real checks, persist user edits across reload, implement every blocked/empty/conflict state, or establish production authentication. Sample issue numbers, commits, times, and check output are fictional. The production app must connect those interactions to the specification's real owners and receipts.

Rebuild the UI using the selected React/shadcn stack. Do not copy the prototype's standalone event delegation or visual-export wrapper as application architecture. Preserve the useful layout and interactions while improving actual responsive behavior, accessibility, and source/conflict states.

The spec is authoritative when the prototype simplifies a flow. Notable examples: the production narrow-screen inspector is a sheet/detail route rather than merely a panel below the list; real GitHub writes expose saving/conflict/unknown states; and current-work landing depends on actual execution state rather than the sample's initial Queue screen.

The concept begins with a populated project. First-run Add repository, managed clone/setup progress, and Add another repository/project switching are explicit required production flows in OP-003/006 and spec §§8–9/21; their absence from the concept does not defer them. INSTALL.md covers host installation separately.
