---
planning_id: OP-004
status: published
order: 4
depends_on: ["OP-003"]
source: bootstrap-handoff
github_issue: 4
---
# OP-004 — Make GitHub Issues the task source

**Dependencies:** #3 (OP-003)  
**Specification:** §§7–10, 18–19, 23, 26.1/3/8/10.  
**Acceptance owners:** AC-07–09, AC-39, AC-45, AC-51–52.

**Canonical task:** https://github.com/felixlheureux/outpost/issues/4  
**Repository copy:** Published planning snapshot; update the issue for current requirements.

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
