# OP-016 release evidence

This map distinguishes executable/fixture checks from owner acceptance. The
October 18, 2026 host checks used Ubuntu 26.04.1, Linux amd64, Go 1.27.2, Node
24.21.0, npm 11.19.0, sqlc 1.31.1, and Codex CLI 0.163.0.
The host's private Outpost service served revision `fff3c03` before release packaging. Do not treat
that local host evidence as a clean-host or Mac/phone journey.

| Acceptance IDs | Current evidence | Remaining acceptance |
| --- | --- | --- |
| AC-01–06 | `internal/cli/install_test.go`, `internal/privateaccess/config_test.go`, `internal/httpapi/server_test.go`, `web/e2e/starter.spec.ts`; actual host private `/healthz`, `/readyz`, `/api/v1/bootstrap`, `/work` responded 200 on `fff3c03` | Fresh clean-host release install, denied second identity, and owner-device HTTPS visit remain unverified. |
| AC-07–11 | `internal/issues/*_test.go`, `internal/queue/queue_test.go`, `internal/scheduler/scheduler_test.go`, `web/e2e/issues.spec.ts`, `web/e2e/queue.spec.ts` | Real separate-session issue creation and queue change remain unverified. |
| AC-12–18 | `internal/store/runs_test.go`, `internal/supervisor/systemd_test.go`, `internal/worker/ownership_test.go`, `internal/validation/run_test.go` | Some systemd tests are opt-in; active owner-run restart and interruption remain unverified. |
| AC-19–24 | `internal/validation/run_test.go`, `internal/delivery/delivery_test.go`, `internal/delivery/report_test.go` | Actual repository check and GitHub delivery/reporting under the owner account remain unverified. |
| AC-25–35 | `internal/httpapi/work_test.go`, `internal/store/work_test.go`, `internal/providerwork/execute_test.go`, `internal/codexadapter/client_test.go`, `internal/httpapi/activity_test.go`, `web/e2e/activity.spec.ts` | Real Codex control, lost acknowledgment, and phone reconnect need owner-device evidence; real provider tests are opt-in. |
| AC-36 | No legacy import implementation. | OP-015 was explicitly waived for this run; historical import provenance remains unverified. |
| AC-37 | `internal/store/store_test.go`, `internal/cli/maintenance_test.go`; `scripts/backup.py` refused an active web service, then backed up the idle host. A disposable restore had SQLite integrity `ok`, schema 11, one project, and one workspace copy. | A complete guarded binary update command and full live restore remain unverified. |
| AC-38–46 | `internal/scheduler/scheduler_test.go`, `internal/delivery/report_test.go`, `internal/queue/queue_test.go`; configured host and real source sync exist. | End-to-end issue → Codex → steer → checks → delivery → report, source addition during work, stop/continue, and reporting outage need an actual owner run. |
| AC-47–52 | `web/e2e/issues.spec.ts`, `web/e2e/queue.spec.ts`, `web/e2e/activity.spec.ts`, `web/e2e/history.spec.ts`, `web/e2e/layout.spec.ts` | Mac/phone and independent GitHub-session evidence remain unverified. |
| AC-53–60 | `internal/projectsetup/setup_test.go`, `internal/projectsetup/existing_test.go`, `web/e2e/onboarding.spec.ts`; one actual managed host clone was retained and recovered from `source_pending` to `ready`. | Empty-state second-repository and coding-slot transfer owner journey remain unverified. |
| AC-61–66 | `make release VERSION=v0.1.0-rc1` built amd64/arm64 binaries with matching checksums; both binary hashes stayed identical across two local builds, and the amd64 binary executed and reported version. `internal/cli/prerequisites_test.go` checks dependency paths. | No GitHub release or clean-host bootstrap exists yet. Arm64 execution, idempotent reinstall, missing-auth path, and final private URL from a fresh install remain unverified. |

The local release build alone does not satisfy OP-016. Do not mark the issue
complete until the remaining account, device, clean-host, migration, and
reporting evidence is recorded. The backup contains private source/state and
must remain outside the repository and release assets.

One idle sample on the configured host, about 65 seconds after a web restart,
showed the `outpost.service` process at 19,308 KiB RSS, 0.0% CPU in `ps`, and
556,649,000 ns accumulated service CPU. The host had one registered repository,
no active run, and an authenticated bootstrap request returned 200. This single
sample does not establish steady-state or workload performance.
