import { expect, test } from "@playwright/test";

test("history keeps old attempts read only and exposes recovery evidence", async ({ page }) => {
  const at = "2026-10-18T12:00:00Z";
  const run = {
    id: "old-run",
    project_id: "p",
    state: "stopped",
    reservation_state: "released",
    launch_observation: "acknowledged",
    schedule_outcome: "blocked",
    schedule_reason: "provider session unavailable",
    created_at: at,
    updated_at: at,
    successful_count: 0,
    task_limit: 1,
    assignment_count: 1,
    assignments: [],
  };
  await page.route("**/api/v1/bootstrap", (route) =>
    route.fulfill({
      json: {
        version: "fixture",
        mode: "development",
        current_run: null,
        capabilities: { repository_setup: true, execution: false },
      },
    }),
  );
  await page.route("**/api/v1/projects", (route) =>
    route.fulfill({
      json: {
        projects: [
          {
            id: "p",
            name: "owner/repo",
            repository_url: "https://github.com/owner/repo",
            setup_state: "ready",
            setup_stage: "ready",
            created_at: at,
          },
        ],
      },
    }),
  );
  await page.route("**/api/v1/history?project=p&offset=0", (route) =>
    route.fulfill({ json: { runs: [run], next_offset: 1 } }),
  );
  await page.route("**/api/v1/history/old-run", (route) =>
    route.fulfill({
      json: {
        ...run,
        assignments: [
          {
            id: "assignment",
            issue_number: 13,
            title: "Pinned task",
            issue_digest: "issue-digest",
            issue_revision: 4,
            queue_sha: "manifest-sha",
            baseline_head: "baseline",
            branch: "main",
            state: "blocked",
            created_at: at,
            settled_at: at,
            attempts: [
              {
                id: "attempt-2",
                ordinal: 2,
                phase: "settled",
                outcome: "blocked",
                provider_state: "failed",
                failure_code: "session_missing",
                model: "fixture",
                reasoning: "low",
                sandbox: "workspace-write",
                approval_policy: "on-request",
                created_at: at,
                settled_at: at,
              },
              {
                id: "attempt-1",
                ordinal: 1,
                phase: "settled",
                outcome: "interrupted",
                provider_state: "interrupted",
                failure_code: "",
                model: "fixture",
                reasoning: "low",
                sandbox: "workspace-write",
                approval_policy: "on-request",
                created_at: at,
                settled_at: at,
              },
            ],
          },
        ],
      },
    }),
  );
  await page.route("**/api/v1/history/old-run/artifacts", (route) =>
    route.fulfill({
      json: {
        artifacts: [
          {
            id: "artifact_attempt-1",
            attempt_id: "attempt-1",
            bytes_written: 80,
            truncated: false,
            retention_state: "retained",
            removed_at: "",
          },
        ],
      },
    }),
  );
  await page.route("**/api/v1/runs/old-run/activity", (route) =>
    route.fulfill({
      json: {
        snapshot: {
          cursor: 1,
          oldest_cursor: 1,
          mode: "",
          instructions: "",
          digest: "",
          state: "accepted",
          reason: "",
          created_at: at,
          updated_at: at,
        },
        events: [
          {
            cursor: 1,
            project_id: "p",
            run_id: "old-run",
            attempt_id: "attempt-1",
            kind: "activity.appended",
            payload: { summary: "Provider interrupted" },
            created_at: at,
          },
        ],
        next_before: 0,
        history_truncated: false,
      },
    }),
  );
  await page.route("**/api/v1/runs/old-run/work", (route) =>
    route.fulfill({
      status: 404,
      json: { error: { code: "not_found", message: "Unavailable", request_id: "fixture" } },
    }),
  );
  await page.route("**/api/v1/diagnostics?project=p", (route) =>
    route.fulfill({
      json: {
        version: "fixture",
        observed_at: at,
        mode: "private",
        private_url: "https://host.tailnet.ts.net",
        allowed_identity: "owner@example.com",
        tailscale: "node matches configuration",
        serve_route: "Outpost route present",
        web_service: "active",
        github: "authenticated",
        provider: "signed in",
        project_setup: "ready",
        issue_source: "ready",
        queue_source: "ready",
        state_access: true,
        disk_available_bytes: 100000000,
        artifacts: { count: 1, recorded_bytes: 80, truncated_count: 0 },
      },
    }),
  );
  await page.goto("/history?project=p");
  await expect(page.getByText("provider session unavailable")).toBeVisible();
  await page.getByRole("button", { name: "View details" }).click();
  await expect(page.getByText("Attempt 2 · blocked")).toBeVisible();
  await expect(page.getByText("Attempt 1 · interrupted")).toBeVisible();
  await expect(page.getByText(/queue revision: manifest-sha/)).toBeVisible();
  await expect(
    page.getByText("Historical receipts are read only. Controls belong to current Work."),
  ).toBeVisible();
  await expect(page.getByRole("button", { name: "Interrupt now" })).toHaveCount(0);
  await page.getByRole("button", { name: "Connection and storage" }).click();
  await expect(page.getByText(/Tailscale: node matches configuration/)).toBeVisible();
  await expect(page.getByText(/Retained output: 1 artifacts/)).toBeVisible();
});
