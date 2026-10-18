# OP-016 release evidence

This map distinguishes executable/fixture checks from owner acceptance. The
owner accepted the remaining manual scenarios for release closeout on October
18, 2026 and directed closure of all open feature issues. That acceptance
changes the closeout decision; it does not turn unexecuted clean-host, second
identity, reboot, migration, or owner-device scenarios into test evidence.

The October 18, 2026 host checks used Ubuntu 26.04.1, Linux amd64, Go 1.27.2, Node
24.21.0, npm 11.19.0, sqlc 1.31.1, and Codex CLI 0.163.0.
The host's private Outpost service served revision `fff3c03` before release packaging. On October 18,
the published `v0.1.0-rc1` amd64 asset was downloaded, its checksum passed, and the installer
upgraded the idle host from a private backup. The binary reported `v0.1.0-rc1`, the web service was
active, and the installer verified the private dashboard URL. The previous executable was retained
in the backup with mode 0700. This is an existing-host upgrade, not clean-host acceptance.
The host was later backed up and upgraded to source revision `2453476` to fix
runner continuation; its private service and existing project records remained available.
The published `v0.1.0-rc3` asset was checksum-verified and installed from an
idle backup on this same designated deployment host. Its unchanged reinstall
preserved two registered projects. Main and release CI passed at `15c0fd0`.
The owner reported that History is visible but its run items lacked spacing;
`v0.1.0-rc4` corrects the run-card gaps and keeps expanded details in a single
History column on desktop. Phone and desktop browser screenshots were inspected,
and all eight browser journeys passed after the change. The `v0.1.0-rc4`
release asset was then checksum-verified and installed on this host from an
idle backup, with the private dashboard verified afterward.
The deployed browser showed four real History run cards; at 390 and 1440 CSS
pixels the measured card gap was 14px and the expanded-section gap was 12px.
The real activity list crowded out receipts, so `v0.1.0-rc5` keeps it in an
expandable, bounded section. Its release asset was then verified and installed
from a fresh idle backup on this same host. A browser against the deployed
private URL returned 200 and showed four run cards, 14px card gaps, and 12px
detail-section gaps at 390 and 1440 CSS pixels. The first run's 99 activity
events were initially collapsed; when opened, the scrolling list stayed 306px
tall at both widths. The installed binary reported `v0.1.0-rc5`, and two
registered projects remained available. The deployed Queue, Work, and History
routes each returned 200 with no document-level horizontal overflow at 320,
390, 768, 820, 1024, and 1440 CSS pixels.

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
| AC-07–11 | `internal/issues/*_test.go`, `internal/queue/queue_test.go`, `internal/scheduler/scheduler_test.go`, `web/e2e/issues.spec.ts`, `web/e2e/queue.spec.ts`; issue #19 was created through GitHub CLI separately from Outpost, appeared in Inbox, and was placed first in the shared queue. | A separate owner-device GitHub session remains unverified. |
| AC-12–18 | `internal/store/runs_test.go`, `internal/supervisor/systemd_test.go`, `internal/worker/ownership_test.go`, `internal/validation/run_test.go`; the opt-in real user-systemd fixture passed on the configured host, including launch replay, web restart, worker survival, observed stop, and slot release. Issue #19's active worker also survived a web-service restart, and issue #21's live provider turn accepted an immediate interrupt and saved-session continuation. | Boot/reboot behavior and owner-device interruption remain unverified. |
| AC-19–24 | `internal/validation/run_test.go`, `internal/delivery/delivery_test.go`, `internal/delivery/report_test.go`; issues #19, #21, #22, and #23 completed configured checks, push delivery, settled GitHub reports, and closure. In private project B, run `run_07059e1630baa7d480be38f7824f7ffe` recorded a real required check failure, a linked Codex repair, then passing marker and whitespace checks before push commit `1b6f80be7ce5461d252b6c4bd075a3f501fb913c`, result report, and issue closure. `TestReportingOutageKeepsLocalSuccessAndRetriesOnlyReport` verifies an unavailable GitHub comment read leaves local success settled and retries only reporting. | Broader repository checks and a live reporting outage remain unverified. |
| AC-25–35 | `internal/httpapi/work_test.go`, `internal/store/work_test.go`, `internal/providerwork/execute_test.go`, `internal/codexadapter/client_test.go`, `internal/httpapi/activity_test.go`, `web/e2e/activity.spec.ts`. On October 18, all four opt-in real Codex tests passed against authenticated Codex CLI 0.163.0 in disposable Git repositories: provider handoff and retained activity, live steer, accepted interrupt, and linked continuation with SQLite receipts. The discovered test model was `gpt-6.1-sol` at low effort; the Work form separately defaults supported models to medium. Live issue #19 used medium reasoning and accepted live direction. Live issue #21 accepted interrupt and saved-session continuation, then passed runner checks and push delivery with a linked attempt. | Lost acknowledgment under real failure and phone reconnect remain unverified. |
| AC-36 | No legacy import implementation. | OP-015 was explicitly waived for this run; historical import provenance remains unverified. |
| AC-37 | `internal/store/store_test.go`, `internal/cli/maintenance_test.go`, `internal/cli/install_test.go`; `scripts/backup.py` refused an active web service, then backed up the idle host. A disposable restore had SQLite integrity `ok`, schema 11, one project, and one workspace copy. The host installer refused an active upgrade, then upgraded to `636043b` from a matching idle backup, preserved the prior binary, and verified the private URL. The published `v0.1.0-rc1` amd64 asset subsequently passed checksum verification and upgraded the same idle host after a fresh backup. | A migration across a changed schema and full live restore remain unverified. |
| AC-38–46 | `internal/scheduler/scheduler_test.go`, `internal/delivery/report_test.go`, `internal/queue/queue_test.go`; issue #19 completed the host/API issue → queue → Codex → direction → checks → delivery → report → closure journey. Issue #21 proved a live interrupt and saved continuation, with the immediate-stop guard retained after task settlement. In run `run_7a6773b2edddb62932ce9e70bea1a335`, #23 was created and queued while #22 was pinned. #22 retained queue SHA `e2c6390233928da4325a1de7d98454d3728a58f6`; after it settled, the same run admitted #23 from SHA `5985ff7688c0a5a6ca2d2f3b436199d35669d055`, then both assignments completed. | Live GitHub reporting outage and owner-device controls remain unverified; the outage retry is covered by a fixture. |
| AC-47–52 | `web/e2e/issues.spec.ts`, `web/e2e/queue.spec.ts`, `web/e2e/activity.spec.ts`, `web/e2e/history.spec.ts`, `web/e2e/layout.spec.ts`; owner reported Work working on Mac and phone and History visible. Browser journeys covered 320, 390, 768, 820, 1024, and 1440 CSS pixels, including History card/section spacing, expandable activity, and no horizontal overflow. Screenshots at 320 and 1440 pixels were inspected. A browser against the deployed private host measured 14px between four real run cards and 12px between expanded sections at 390 and 1440 pixels. Error colors measured 6.47:1 in light mode and 6.40:1 in dark mode against the card surfaces; the dark destructive button measured 8.31:1. | Complete phone control and independent GitHub-session evidence remain unverified. |
| AC-53–60 | `internal/projectsetup/setup_test.go`, `internal/projectsetup/existing_test.go`, `web/e2e/onboarding.spec.ts`; the opt-in real GitHub test passed against the private disposable repository, including managed clone, source observation, `.git` replay, and advanced existing-checkout registration. One actual managed host clone was retained and recovered from `source_pending` to `ready`. On October 18, 2026, the private second repository `felixlheureux/outpost-owner-acceptance-2026` was added through the installed Outpost URL setup, producing a distinct ready managed checkout and initializing an empty planning queue without changing its main branch. Repeating the URL with different case and a `.git` suffix reused the same project without cloning again. The live A/B coding-slot transfer recorded above rejected B with `slot_busy` while A was active, then ran B after A settled; browsing A continued to identify B as the global active project. | Empty-state onboarding and complete owner-device checks remain outstanding; clean-host acceptance remains unverified. |
| AC-61–66 | `make release VERSION=v0.1.0-rc1` built amd64/arm64 binaries with matching checksums; both binary hashes stayed identical across two local builds, and the amd64 binary executed and reported version. Clean GitHub Actions builds for main revision `15c0fd0`, tag `v0.1.0-rc3`, tag `v0.1.0-rc4`, and the final `v0.1.0-rc5` tag passed source generation, Go/frontend checks, browser smoke, and release packaging. The published prerelease has amd64/arm64 binaries, checksums, and the backup utility. `internal/cli/prerequisites_test.go` checks dependency paths. The `v0.1.0-rc3`, `v0.1.0-rc4`, and `v0.1.0-rc5` amd64 assets were installed on the designated host after checksum verification and private backups; rc3 was reinstalled unchanged and retained two projects. An unavailable Codex PATH stopped an earlier upgrade before replacement, with the old service restored. On the configured host, `GH_CONFIG_DIR` pointed at an empty private directory made `outpost doctor --repo` exit nonzero with GitHub and repository access remedies. A fresh Ubuntu 26.04 container correctly refused root installation and reported missing systemd for an ordinary user without claiming readiness. | Separate clean-host installation, arm64 execution, missing-auth path from a clean host, and the final URL from a fresh install remain unverified. |

The owner designated the current host as the deployment target and accepted
the remaining manual checks for release closeout. A separate clean-host install
was not a deployment prerequisite and remains unverified. Do not label
other missing account/device, migration, or reporting evidence as verified.
The backup contains private source/state and
must remain outside the repository and release assets.

One idle sample on the configured host, about 65 seconds after a web restart,
showed the `outpost.service` process at 19,308 KiB RSS, 0.0% CPU in `ps`, and
556,649,000 ns accumulated service CPU. The host had one registered repository,
no active run, and an authenticated bootstrap request returned 200. This single
sample does not establish steady-state or workload performance.
