import { expect, test } from "@playwright/test";

test("activity reconnect preserves the instructions draft and opened reader", async ({ page }) => {
  const at = "2026-10-18T12:00:00Z";
  await page.route("**/api/v1/bootstrap", (route) =>
    route.fulfill({
      json: {
        version: "test",
        mode: "development",
        current_run: { id: "run_1", project_id: "p", state: "running", liveness: "active" },
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
  await page.route("**/api/v1/projects/p/issues", (route) =>
    route.fulfill({
      json: { source: { state: "ready", last_success_at: at, last_error: "" }, issues: [] },
    }),
  );
  await page.route("**/api/v1/runs/run_1/activity", (route) =>
    route.fulfill({
      json: {
        snapshot: {
          cursor: 2,
          oldest_cursor: 1,
          run_id: "run_1",
          project_id: "p",
          run_state: "running",
          assignment_id: "a",
          attempt_phase: "implementing",
          provider_state: "turn_started",
          model: "fixture",
          reasoning: "low",
          sandbox: "workspace-write",
          approval_policy: "on-request",
        },
        events: [
          {
            cursor: 2,
            project_id: "p",
            run_id: "run_1",
            attempt_id: "attempt",
            kind: "activity.appended",
            payload: { category: "message", summary: "First message" },
            created_at: at,
          },
        ],
        next_before: 2,
        history_truncated: false,
      },
    }),
  );
  await page.route("**/api/v1/runs/run_1/activity?before=2", (route) =>
    route.fulfill({
      json: {
        snapshot: {
          cursor: 2,
          oldest_cursor: 1,
          run_id: "run_1",
          project_id: "p",
          run_state: "running",
        },
        events: [
          {
            cursor: 1,
            project_id: "p",
            run_id: "run_1",
            attempt_id: "attempt",
            kind: "activity.appended",
            payload: { category: "message", summary: "Older message" },
            created_at: at,
          },
        ],
        next_before: 0,
        history_truncated: false,
      },
    }),
  );
  let streams = 0;
  await page.route("**/api/v1/activity/stream?**", (route) => {
    streams += 1;
    const id = streams === 1 ? 3 : 4;
    const message =
      streams === 1 ? "Second message" : 'Third message <img src=x onerror="alert(1)">';
    void route.fulfill({
      status: 200,
      contentType: "text/event-stream",
      body: `id: ${id}\nevent: activity.appended\ndata: ${JSON.stringify({ cursor: id, project_id: "p", run_id: "run_1", attempt_id: "attempt", kind: "activity.appended", payload: { category: "message", summary: message }, created_at: at })}\n\n`,
    });
  });
  await page.goto("/work?project=p");
  const draft = page.getByLabel("Instructions draft");
  await draft.fill("Keep the unfinished note while I read.");
  await page.getByRole("button", { name: "Load older activity" }).click();
  await expect(page.getByText("Older message")).toBeVisible();
  await expect(page.getByText("Second message")).toBeVisible();
  await page.getByRole("button", { name: /Older message/ }).click();
  await expect(page.getByRole("dialog")).toContainText("Older message");
  await expect(page.getByText(/Third message <img/)).toBeVisible({ timeout: 10_000 });
  await expect(page.locator(".activity-entry img")).toHaveCount(0);
  await expect(page.getByRole("dialog")).toContainText("Older message");
  await page.keyboard.press("Escape");
  await page.getByRole("link", { name: "Queue" }).click();
  await page.getByRole("link", { name: "Work", exact: true }).click();
  await expect(page.getByLabel("Instructions draft")).toHaveValue(
    "Keep the unfinished note while I read.",
  );
});
