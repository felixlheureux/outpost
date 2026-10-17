---
planning_id: OP-003
status: published
order: 3
depends_on: ["OP-001", "OP-002"]
source: bootstrap-handoff
github_issue: 3
---
# OP-003 — Add repositories with managed cloning and build the project shell

**Dependencies:** #1 (OP-001), #2 (OP-002)  
**Specification:** §§8.1, 9, 18–19, 21.1, 22, 25.  
**Acceptance owners:** AC-11, AC-13, AC-53–54, AC-57–58, AC-60; shell foundation for AC-55–56.

**Canonical task:** https://github.com/felixlheureux/outpost/issues/3  
**Repository copy:** Published planning snapshot; update the issue for current requirements.

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


### Starter integration and simple authentication

Read docs/starter-handoff.md, docs/architecture.md and docs/dependencies.md. The embedded Go/React foundation already exists. Extend it rather than regenerating the application. Use native Git for checkouts and the existing internal/githubcli adapter for authenticated gh API calls. Normal installation reuses `gh auth login --hostname github.com --web --git-protocol https`, `gh auth setup-git` where appropriate, Codex account sign-in, and Tailscale sign-in. Do not require copied API keys, a PAT, GitHub OAuth app registration, or an Outpost account. Reuse valid sessions and preserve existing SSH choices. Run setup as the eventual ordinary service user; verify that its credential store works in that service environment. Human sign-in remains a guided resumable step. Do not try to open login dialogs from a background HTTP command.
