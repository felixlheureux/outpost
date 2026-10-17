---
planning_id: OP-016
status: published
order: 16
depends_on: ["OP-001", "OP-002", "OP-003", "OP-004", "OP-005", "OP-006", "OP-007", "OP-008", "OP-009", "OP-010", "OP-011", "OP-012", "OP-013", "OP-014", "OP-015"]
source: bootstrap-handoff
github_issue: 16
---
# OP-016 — Package the release and prove the complete owner workflow

**Dependencies:** #1 (OP-001), #2 (OP-002), #3 (OP-003), #4 (OP-004), #5 (OP-005), #6 (OP-006), #7 (OP-007), #8 (OP-008), #9 (OP-009), #10 (OP-010), #11 (OP-011), #12 (OP-012), #13 (OP-013), #14 (OP-014), #15 (OP-015)  
**Specification:** §§24–25, 28–29, 33–34.  
**Acceptance owners:** Final integration of AC-01 through AC-52, especially AC-37–38.

**Canonical task:** https://github.com/felixlheureux/outpost/issues/16  
**Repository copy:** Published planning snapshot; update the issue for current requirements.

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
