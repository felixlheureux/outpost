# OP-016 release evidence

This map distinguishes executable/fixture checks from owner acceptance. The
October 18, 2026 host checks used Ubuntu 26.04.1, Linux amd64, Go 1.27.2, Node
24.21.0, npm 11.19.0, sqlc 1.31.1, and Codex CLI 0.163.0.
The host's private Outpost service served revision `fff3c03` before release packaging. On October 18,
the published `v0.1.0-rc1` amd64 asset was downloaded, its checksum passed, and the installer
upgraded the idle host from a private backup. The binary reports `v0.1.0-rc1`, the web service is
active, and the installer verified the private dashboard URL. The previous executable was retained
in the backup with mode 0700. This is an existing-host upgrade, not clean-host acceptance.

On October 18, 2026, the first live Outpost owner run,
`run_b0c92818d8a333da5238cf53c99f3632`, completed issue #19. The issue was seen
in Inbox and placed first in the shared queue. The medium-reasoning Codex turn
accepted live direction and continued through a web-service restart, the
operator-owned profile's configured whitespace check, and push delivery at
commit `401c1ce7265652fdfb52ed822aa70ecaa2693f2d`. The GitHub result report
settled, followed by issue closure. This records the host/API journey; complete
owner phone control and clean-host acceptance remain unverified.

On October 18, 2026, the live two-repository coding-slot transfer recorded in
issue #23 used A (`felixlheureux/outpost`) and B
(`felixlheureux/outpost-owner-acceptance-2026`). While A owned an active run,
a start request for B returned `slot_busy` and no second worker started.
After A settled, B ran its own issue #1 in
`run_c49e14c11e3e17517b93f29c5729dde8` through the configured check, push delivery
at commit `85e1174cb124a62b2406a53816aa50296aa9639a`, a settled GitHub result
report, and issue closure. Browsing A while B executed still identified B as
the global active project. This records the host/API coding-slot transfer;
complete owner phone/device control and clean-host acceptance remain unverified.

| Acceptance IDs | Current evidence | Remaining acceptance |
| --- | --- | --- |
| AC-01–06 | `internal/cli/install_test.go`, `internal/privateaccess/config_test.go`, `internal/httpapi/server_test.go`, `web/e2e/starter.spec.ts`; actual host private `/healthz`, `/readyz`, `/api/v1/bootstrap`, `/work` responded 200 on `fff3c03`. Owner reported the deployed Work page working on both Mac and phone over Tailscale on October 18, 2026. | Fresh clean-host release install and denied second identity remain unverified. |
| AC-07–11 | `internal/issues/*_test.go`, `internal/queue/queue_test.go`, `internal/scheduler/scheduler_test.go`, `web/e2e/issues.spec.ts`, `web/e2e/queue.spec.ts`; live issue #19 was seen in Inbox and placed first in the shared queue as recorded above. | Separate-session issue creation remains unverified. |
| AC-12–18 | `internal/store/runs_test.go`, `internal/supervisor/systemd_test.go`, `internal/worker/ownership_test.go`, `internal/validation/run_test.go`; live issue #19 continued through a web-service restart. | Some systemd tests are opt-in; active owner-run interruption remains unverified. |
| AC-19–24 | `internal/validation/run_test.go`, `internal/delivery/delivery_test.go`, `internal/delivery/report_test.go`; live issue #19 completed the configured whitespace check, push delivery, settled GitHub report, and closure as recorded above. | Broader repository checks and reporting-outage recovery remain unverified in the owner journey. |
| AC-25–35 | `internal/httpapi/work_test.go`, `internal/store/work_test.go`, `internal/providerwork/execute_test.go`, `internal/codexadapter/client_test.go`, `internal/httpapi/activity_test.go`, `web/e2e/activity.spec.ts`. On October 18, all four opt-in real Codex tests passed against authenticated Codex CLI 0.163.0 in disposable Git repositories: provider handoff and retained activity, live steer, accepted interrupt, and linked continuation with SQLite receipts. The discovered test model was `gpt-6.1-sol` at low effort; the Work form separately defaults supported models to medium. Live issue #19 used medium reasoning and accepted live direction. | Lost acknowledgment under real failure and phone reconnect remain unverified. |
| AC-36 | No legacy import implementation. | OP-015 was explicitly waived for this run; historical import provenance remains unverified. |
| AC-37 | `internal/store/store_test.go`, `internal/cli/maintenance_test.go`, `internal/cli/install_test.go`; `scripts/backup.py` refused an active web service, then backed up the idle host. A disposable restore had SQLite integrity `ok`, schema 11, one project, and one workspace copy. The host installer refused an active upgrade, then upgraded to `636043b` from a matching idle backup, preserved the prior binary, and verified the private URL. The published `v0.1.0-rc1` amd64 asset subsequently passed checksum verification and upgraded the same idle host after a fresh backup. | A migration across a changed schema and full live restore remain unverified. |
| AC-38–46 | `internal/scheduler/scheduler_test.go`, `internal/delivery/report_test.go`, `internal/queue/queue_test.go`; live issue #19 completed the host/API issue → queue → Codex → direction → checks → delivery → report → closure journey recorded above. | Source addition during work, stop/continue, and reporting outage still need an actual owner run. |
| AC-47–52 | `web/e2e/issues.spec.ts`, `web/e2e/queue.spec.ts`, `web/e2e/activity.spec.ts`, `web/e2e/history.spec.ts`, `web/e2e/layout.spec.ts`; owner reported the Work page working on Mac and phone. | Complete phone control and independent GitHub-session evidence remain unverified. |
| AC-53–60 | `internal/projectsetup/setup_test.go`, `internal/projectsetup/existing_test.go`, `web/e2e/onboarding.spec.ts`; one actual managed host clone was retained and recovered from `source_pending` to `ready`. On October 18, 2026, the private second repository `felixlheureux/outpost-owner-acceptance-2026` was added through the installed Outpost URL setup, producing a distinct ready managed checkout and initializing an empty planning queue without changing its main branch. Repeating the URL with different case and a `.git` suffix reused the same project without cloning again. The live A/B coding-slot transfer recorded above rejected B with `slot_busy` while A was active, then ran B after A settled; browsing A continued to identify B as the global active project. | Empty-state onboarding and complete owner-device checks remain outstanding; clean-host acceptance remains unverified. |
| AC-61–66 | `make release VERSION=v0.1.0-rc1` built amd64/arm64 binaries with matching checksums; both binary hashes stayed identical across two local builds, and the amd64 binary executed and reported version. Clean GitHub Actions builds for main revision `0eb3410` and tag `v0.1.0-rc1` passed source generation, Go/frontend checks, browser smoke, and release packaging. The published prerelease has amd64/arm64 binaries, checksums, and the backup utility. `internal/cli/prerequisites_test.go` checks dependency paths. The host install was rerun unchanged on `636043b`; it retained one ready project and one cached queue/source, then verified the private URL. The published amd64 asset was installed after checksum verification and again verified the private URL. An unavailable Codex PATH stopped an earlier upgrade before replacement, with the old service restored. | No clean-host bootstrap exists yet. Arm64 execution, missing-auth path from a clean host, and the final URL from a fresh install remain unverified. |

The local release build alone does not satisfy OP-016. Do not mark the issue
complete until the remaining account, device, clean-host, migration, and
reporting evidence is recorded. The backup contains private source/state and
must remain outside the repository and release assets.

One idle sample on the configured host, about 65 seconds after a web restart,
showed the `outpost.service` process at 19,308 KiB RSS, 0.0% CPU in `ps`, and
556,649,000 ns accumulated service CPU. The host had one registered repository,
no active run, and an authenticated bootstrap request returned 200. This single
sample does not establish steady-state or workload performance.
