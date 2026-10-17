# Outpost — Implementation Tasks

**Revision:** 0.2 · October 17, 2026 · Published to felixlheureux/outpost  
**Target:** First usable personal release on a Linux host over Tailscale  
**Companion specification:** [Outpost-Technical-Specification.md](Outpost-Technical-Specification.md)  
**Interactive design reference:** [Outpost-UX-Reference.html](Outpost-UX-Reference.html)

## How to use this backlog

Read the specification's confirmed decisions, authority model, and acceptance strategy before implementation. Then work on the next dependency-ready task in the order below. Each task is an outcome with concrete acceptance, not a suggestion to build every future extension.

The task files in `tasks/` retain stable `OP-xxx` planning IDs. All 16 tasks are now published in [felixlheureux/outpost](https://github.com/felixlheureux/outpost/issues); `issue-map.json` records their actual issue identities. The live `queue.json` is on `outpost/queue` and uses real issue numbers. These repository task files and this document are published reference snapshots; update GitHub Issues for current requirements and the planning branch for current order.

If the repository already contains implementation, inspect it and its `AGENTS.md`/documentation first. Reuse working code, mark demonstrated acceptance, and implement the remaining behavior. This package does not claim every component must be rewritten or that an empty repository exists.

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
| 3 | OP-003 | OP-001, OP-002 | Project registration, readiness, and compact application shell |
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
**Acceptance owners:** AC-05, AC-06, foundation for AC-37.

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

### Failure and edge cases

Unsupported newer schema fails clearly. A half-built frontend cannot pass the packaging check. Direct navigation to a frontend route succeeds; a typo in an API/asset route remains a 404. Runtime state directories are outside the source checkout and receive private permissions.

### Acceptance criteria

- [ ] A clean documented command produces the executable and actual UI assets.
- [ ] Launching that executable without a dashboard Node/Vite process serves UI and API.
- [ ] A direct frontend-route refresh succeeds while missing API/assets remain 404.
- [ ] Fresh migrations apply to a temporary SQLite database and reject an unsupported schema.
- [ ] Typecheck, production build, and a release-binary HTTP smoke check pass.

### Verification and handoff

Use one focused routing/migration integration test and one embedded-binary smoke journey. Report executable size and idle memory only if actually measured, with the environment stated. Handoff includes build commands, state-path convention, API-error shape, migration ownership, and where later services plug into the shell.

## OP-002 — Make Tailscale access and supervised execution real

**Dependencies:** OP-001.  
**Specification:** §§6, 12, 15, 22–23, 25.  
**Acceptance owners:** AC-01–04, AC-12–17, AC-25–26 at supervision level.

### Outcome

The application is reachable privately through Tailscale, and a supervised fixture worker keeps running when the web service or browser goes away. Ownership uncertainty cannot launch a second writer.

### Implementation requirements

1. Add installation/configuration and user-systemd integration. Separate the long-lived web unit from uniquely named run worker units invoking the same executable with a run identity.
2. Inspect current Tailscale status/Serve configuration and select a route without replacing unrelated routes. Bind Echo to loopback. Print the actual private HTTPS address and allowed identity after verified setup.
3. Trust Tailscale identity headers only through the configured local access boundary; deny missing/disallowed identities across HTML, API, SSE, and artifacts. Add exact-origin/CSRF protections to mutations. Do not open a public port or enable Funnel.
4. Persist run reservation/launch intent before D-Bus effects. Record unique run token/unit identity and check worker ownership before repository or agent effects. Do not hold a database transaction during systemd calls.
5. Reconcile missing/lost launch acknowledgements by observing the recorded unit and process identity. Replayed starts return the existing receipt; duplicate worker claims fail safely.
6. Define worker/check process-group settlement and OS workspace-lock lifetime. An owned surviving child retains exclusion even if its parent disappears. Normal recovery uses observed ownership, never deleting a lock file.
7. Add install/status/stop diagnostics for absent user bus, disabled lingering, unavailable Tailscale, unavailable CLI prerequisites, unwritable directories, and unknown owned processes. Keep requested host changes explicit and narrow.

### Failure and edge cases

Browser disconnection proves nothing about worker state. A successful systemd job acknowledgement does not prove task completion. A web restart must not kill the worker; a VM reboot must not silently resume an arbitrary effect. Keep reservation until known worker/check ownership is reconciled.

### Acceptance criteria

- [ ] A real user-systemd fixture survives web-service restart and can be observed/stopped afterward.
- [ ] Concurrent/replayed starts establish one reservation and one worker.
- [ ] Lost launch acknowledgement attaches to the existing unit rather than launching twice.
- [ ] A surviving check fixture prevents another writer and exposes a relevant stop action.
- [ ] Existing Serve routes survive installation; allowed/private access succeeds and denied identity/origin controls fail.
- [ ] Closing the browser leaves execution unaffected.

### Verification and handoff

Use deterministic local process fixtures for ownership races, then a real Linux user-systemd/Tailscale smoke journey. Label unavailable host checks explicitly. Handoff exposes the supervisor interface, reservation lifecycle, process identity receipt, and current liveness projection for the scheduler and UI.

## OP-003 — Register projects and build the compact application shell

**Dependencies:** OP-001, OP-002.  
**Specification:** §§8–9, 19, 21–22, 25.  
**Acceptance owners:** AC-11, AC-13, initial AC-47/50.

### Outcome

The owner can register existing repository workspaces, inspect readiness, switch projects, and navigate Queue, Work, and History without losing the current run.

### Implementation requirements

1. Store project identity, canonical workspace/Git common directory, execution defaults, configured check profiles, Git delivery target, and source configuration references. Do not put credentials in public DTOs.
2. Inspect existing checkout/branch/remote, staged/unstaged/untracked paths, and provider prerequisites. Registration does not clone, reset, stash, switch branches, rewrite remotes, or authenticate the agent silently.
3. Detect filesystem aliases and duplicate checkout identities. A second project record cannot bypass workspace exclusion by referring to the same location differently.
4. Provide readiness with machine codes and concrete next actions for unavailable provider, dirty/unassigned work, changed baseline, missing profiles, missing source setup, and owned worker/check. Mark source-dependent readiness incomplete until OP-004/005 provides it.
5. Build the prototype's restrained shell: narrow desktop navigation for Queue/Work/History, project selection only when multiple projects exist, secondary project settings and connection diagnostics, and a persistent run bar that names its project/task.
6. Default landing is Work during active/input-required work, Queue when idle; honor deep links and deliberate navigation. Switching projects does not retarget an active run's controls or instruction draft.
7. Show actual empty/loading/unavailable states. The global run bar must use real server observations, distinguish stale connection from stopped execution, and link to the owning run even while another project's queue is open.

### Failure and edge cases

Unknown paths and non-Git directories produce readable errors. Archiving a project with unresolved work is rejected. Unsupported provider setup is a readiness reason, not permission to invent a model or claim the installation is ready.

### Acceptance criteria

- [ ] Two independent repositories can be registered and selected with separate settings.
- [ ] Alias registration cannot create competing ownership of the same checkout.
- [ ] Registration leaves Git state unchanged.
- [ ] The run bar remains correct through project/route changes.
- [ ] Readiness explains the failed prerequisite and an appropriate next action.
- [ ] Empty and populated shell layouts remain usable on desktop and a narrow phone.

### Verification and handoff

Use temporary real Git repositories for identity/baseline observations and one shell navigation journey. Handoff specifies project/readiness DTOs, settings revisions, source-configuration slots, and current-run navigation used by subsequent tasks.

## OP-004 — Make GitHub Issues the task source

**Dependencies:** OP-003.  
**Specification:** §§7–10, 18–19, 23, 26.1/3/8/10.  
**Acceptance owners:** AC-07–09, AC-39, AC-45, AC-51–52.

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

### Failure and edge cases

An inaccessible issue is not automatically missing/deleted. Authentication/rate limits remain distinguishable. No webhook, public callback, or direct ChatGPT-to-tailnet connection is required. A source refresh never replaces the active assignment.

### Acceptance criteria

- [ ] A separately created ordinary GitHub Issue appears in the local source projection after sync.
- [ ] Template/default metadata parsing produces a stable task objective and effective policy.
- [ ] Invalid/cyclic metadata blocks admission with the exact issue and reason.
- [ ] Create/edit success and known/unknown failure have inspectable receipts.
- [ ] Detected edit conflicts preserve the local draft; no atomic issue-update guarantee is fabricated.
- [ ] Closed/edited issues do not fabricate checks or silently rerun completed work.

### Verification and handoff

Use GitHub fixtures for pagination, auth/rate failures, malformed bodies, creation timeouts, and conflicts; one real configured-repository read/create/edit journey is separate acceptance. Handoff provides source DTOs/digests, parsing contract, operation receipt API, and reconciliation hooks.

## OP-005 — Implement the shared planning-branch queue

**Dependencies:** OP-004.  
**Specification:** §§8, 18–19, 26.1–6.  
**Acceptance owners:** AC-40–41, AC-44–45, AC-52.

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

### Failure and edge cases

Completed/closed entries may remain in the list and are skipped with provenance; removal is an explicit planning action. Removing an active issue does not interrupt its pinned assignment. Shared queue storage does not create distributed execution locking across separate installations.

### Acceptance criteria

- [ ] One reorder creates one manifest change and no issue edits or checkout changes.
- [ ] Two stale clients cannot silently overwrite each other; conflict preserves intended order.
- [ ] Invalid versions, duplicates, missing/inaccessible references, and source outages are explained.
- [ ] An uncertain write is inspected before a retry.
- [ ] Partial creation/append failure queues the same created issue on recovery.
- [ ] The current manifest SHA and source-health state are available to scheduler/UI.

### Verification and handoff

Use a focused fake GitHub contents service for SHA races/uncertainty and one real planning-branch read/write acceptance. Record checkout HEAD/status before and after to prove queue operations did not change it. Handoff documents manifest schema, error/receipt DTOs, and atomicity limits.

## OP-006 — Build Inbox, Queue, the task inspector, and start panel

**Dependencies:** OP-003, OP-004, OP-005.  
**Specification:** §21.1–6 and §§8, 10, 19.  
**Acceptance owners:** AC-07–08, AC-39–41, AC-45, AC-47.

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

### Failure and edge cases

Selection survives a source reorder; removal/closure of the selected issue has an explicit state. Two-tab conflicts keep both source and local intent inspectable. A failed append must not invite another Create issue click as the only recovery path.

### Acceptance criteria

- [ ] Create → Inbox → inspect/edit → queue → move works against the backend contracts.
- [ ] Inspector navigation preserves list context and unsaved form text.
- [ ] Queue position and eligibility are distinguishable, including dependencies and holds.
- [ ] Stale SHA/source edit conflicts preserve drafts and never claim a rejected write was saved.
- [ ] The start panel makes its scope and effective settings clear without a second mandatory approval screen.
- [ ] Keyboard and basic narrow-screen operations work before the later responsive pass.

### Verification and handoff

One browser journey covers creation, partial failure, reorder conflict, and task selection; avoid component snapshots mirroring markup. Handoff identifies the real start request payload and the selected issue/source revision used by the scheduler.

## OP-007 — Add durable scheduling, Follow queue, and task admission

**Dependencies:** OP-002, OP-003, OP-005.  
**Specification:** §§9–12, 18–19, 26.  
**Acceptance owners:** AC-09–16, AC-25, AC-40–44, AC-51.

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

### Failure and edge cases

External issue closure is not check evidence. A changed checkout may invalidate predecessor availability. Queue outage lets active work finish but blocks the next admission. A drained run settles rather than polling forever. A run reservation is not released just because a display status looks terminal.

### Acceptance criteria

- [ ] With `[72,41,88]` and `72→41`, the first eligible task is 41 and explanations remain visible.
- [ ] A newly queued task runs at the next eligible boundary within budget, while the active task stays pinned.
- [ ] Fixed scope excludes additions and both modes honor source outages/stop/baseline checks.
- [ ] Concurrent starts/control races produce one writer and no unwanted next task.
- [ ] Failed assigned work is preserved and not abandoned for another task.
- [ ] Completion/closure/reopen behavior preserves execution generations and provenance.

### Verification and handoff

Use real SQLite concurrency, temporary Git baselines, and a deterministic fixture worker. Exercise source-change/admission timing and stop-after-task races. Handoff includes the assignment DTO, transition guards, next-eligible projection, and provider lifecycle integration point.

## OP-008 — Integrate the real Codex App Server adapter

**Dependencies:** OP-007.  
**Specification:** §§11–14, 23, 28.  
**Acceptance owners:** AC-16, AC-26–31 at adapter level, real-provider portion of AC-38.

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

### Failure and edge cases

Malformed/oversized frames, stderr spam, lost terminal acknowledgement, missing executable, authentication expiry, and unexpected process exit all preserve task assignment and receipts. Avoid automatically retrying a turn whose effects are unknown.

### Acceptance criteria

- [ ] Fixture protocol sessions prove start/events/result/error/cancellation behavior and resource bounds.
- [ ] Actual discovered settings are visible and stored with the attempt.
- [ ] Saved-session failure never silently starts a fresh conversation.
- [ ] A real authenticated Codex task performs a small disposable repository change with recorded identities.
- [ ] Agent claims are not marked runner-verified checks or delivery.

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

**Dependencies:** OP-006, OP-010, OP-013.  
**Specification:** §21, especially 21.11–12; §22.  
**Acceptance owners:** AC-32, AC-47–50.

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

### Failure and edge cases

Use a long task title, long unbroken branch/path, empty Inbox, source conflict, stop-pending state, and unknown message receipt. Test a dialog opened after scrolling and a draft typed before a route change.

### Acceptance criteria

- [ ] Every target width has readable primary content without page overflow or squeezed fixed columns.
- [ ] Phone operational flows are reachable with the keyboard open and touch controls usable.
- [ ] Keyboard users can inspect/edit/move tasks, send instructions, and close dialogs with sensible focus return.
- [ ] Reconnection/navigation preserves scoped drafts and reading position.
- [ ] Light/dark/status/focus contrast passes the chosen accessibility checks.

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

**Dependencies:** OP-001 through OP-015.  
**Specification:** §§24–25, 28–29, 33–34.  
**Acceptance owners:** Final integration of AC-01 through AC-52, especially AC-37–38.

### Outcome

The first release can be built, installed, backed up, updated, and used for real GitHub-backed Codex work from the owner's Mac and phone over Tailscale.

### Implementation requirements

1. Produce a documented reproducible release build with frontend assets/migrations embedded, pinned dependencies, version metadata, and platform target. Keep Git/Codex/Tailscale/systemd/repository toolchains explicit host prerequisites.
2. Document idle/maintenance backup of database and referenced artifacts/config, excluding careless token exposure. Coordinate schema migration with owned workers/checks; reject unsupported newer databases and unsafe active migration. Restore a representative backup.
3. Complete the actual sequence: inspect pushed code in a separate GitHub session; create an issue; observe Inbox; queue/reorder; start Follow queue; perform real Codex work; send direction; finish/check/deliver; observe completion reporting; inspect History from another device.
4. Demonstrate source addition during execution affecting the next boundary and pinning the active task. Demonstrate a safe stop/continuation and a source/reporting interruption without duplicate coding work.
5. Restart the web service during active work and reconnect. Separately exercise the specified process/receipt failure paths with fixtures; do not require deliberately destructive failure against valuable live work.
6. Map AC-01 through AC-52 to existing focused test/journey evidence. Combine related cases rather than creating 52 standalone tests. Identify unavailable real-account/host/device evidence honestly.
7. Run final formatting/generation before the agreed relevant gate: Go vet/tests, frontend typecheck/build, required browser journeys, sqlc freshness, and embedded-binary smoke. Use race/host checks where changed concurrency/supervision warrants them. Do not restart unchanged full gates after documentation-only evidence edits.
8. Record measured idle/resource behavior with host/workload context if available. Do not claim Go memory/speed advantages from language choice alone.
9. Finalize operator quickstart, GitHub credential/source setup, task template, queue conflict/recovery guide, checks/delivery configuration, stop semantics, logs/retention, backup/update, and troubleshooting.

### Failure and edge cases

Fixture success cannot substitute for actual Codex/Tailscale/device acceptance. Pending reporting is a visible limitation, not an excuse to mark a fresh run active. A release with simulated activity or no real worker control is incomplete.

### Acceptance criteria

- [ ] The standalone executable serves the real application without a dashboard Node/Vite runtime.
- [ ] The owner completes a real issue → queue → Codex → direction → checks → delivery → GitHub report journey over Tailscale.
- [ ] Mac/phone revisit, web restart, stop/continuation, and pending reporting remain usable and truthful.
- [ ] Backup/restore and guarded update/migration preserve records.
- [ ] The acceptance map names evidence and any remaining limitations explicitly.
- [ ] No deferred platform features are required to use the release.

### Verification and final handoff

Provide release/build identity, installation address, tested tool/provider versions, concise acceptance map, measured resource notes if any, known limitations, and upgrade/backup commands. Keep secrets and private logs out of public issue reports. Deferred work remains a separate backlog rather than expanding this release indefinitely.

## Prototype reference and deliberate limits

Open `Outpost-UX-Reference.html` in a browser. It is a standalone exported reference with illustrative tasks and in-memory interactions. Its Queue/Work/History navigation, task selection/movement, issue-form shape, Activity/Changes/Checks, stop/interrupt/continue, and contextual composer help explain the intended experience.

The reference does **not** call GitHub, launch Codex/systemd, perform real checks, persist user edits across reload, implement every blocked/empty/conflict state, or establish production authentication. Sample issue numbers, commits, times, and check output are fictional. The production app must connect those interactions to the specification's real owners and receipts.

Rebuild the UI using the selected React/shadcn stack. Do not copy the prototype's standalone event delegation or visual-export wrapper as application architecture. Preserve the useful layout and interactions while improving actual responsive behavior, accessibility, and source/conflict states.

The spec is authoritative when the prototype simplifies a flow. Notable examples: the production narrow-screen inspector is a sheet/detail route rather than merely a panel below the list; real GitHub writes expose saving/conflict/unknown states; and current-work landing depends on actual execution state rather than the sample's initial Queue screen.
