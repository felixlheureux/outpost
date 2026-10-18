import { expect, test } from "@playwright/test";

test("activity reconnect preserves the instructions draft and opened reader", async ({ page }) => {
  const at = "2026-10-18T12:00:00Z";
  let providerState = "turn_started";
  let attemptID = "attempt";
  let stopRequested = false;
  let messageSubmissions = 0;
  let messages: Record<string, unknown>[] = [];
  let controls: Record<string, unknown>[] = [];
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
  await page.route("**/api/v1/runs/run_1/work", (route) =>
    route.fulfill({
      json: {
        run_id: "run_1",
        project_id: "p",
        run_state: "running",
        assignment_id: "a",
        attempt_id: attemptID,
        attempt_phase: "implementing",
        attempt_outcome: "running",
        provider_state: providerState,
        thread_id: "thread",
        turn_id: "turn",
        turn_open: providerState === "turn_started",
        issue_number: 9,
        snapshot: {
          issue_number: 9,
          title: "Build Work controls",
          body: "Pinned task brief",
          issue_url: "https://github.com/owner/repo/issues/9",
        },
        model: "fixture",
        reasoning: "low",
        sandbox: "workspace-write",
        approval_policy: "on-request",
        successful_count: 0,
        task_limit: 2,
        stop_requested: stopRequested,
        schedule_outcome: "active",
        schedule_reason: "",
        messages,
        controls,
        checks: [
          {
            id: "check_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
            check_id: "test",
            name: "Tests",
            required: true,
            status: "failed",
            reason: "zero or unknown selected cases",
            input_digest: "abcdef123456",
            candidate_digest: "fedcba654321",
            contract_digest: "112233445566",
            duration_ms: 152,
            exit_code: 0,
            signal: "",
            case_count: 0,
            require_cases: true,
            started_at: at,
            finished_at: at,
            pid: 44,
            output_bytes: 9,
            truncated: false,
          },
        ],
      },
    }),
  );
  await page.route(
    "**/api/v1/runs/run_1/checks/check_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa/output",
    (route) =>
      route.fulfill({
        json: {
          id: "check_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
          output: "0 tests\n",
          truncated: false,
        },
      }),
  );
  await page.route("**/api/v1/runs/run_1/messages", async (route) => {
    messageSubmissions += 1;
    if (messageSubmissions === 1) {
      await route.fulfill({
        status: 409,
        json: {
          error: {
            code: "stale_target",
            message: "The targeted turn changed. Your draft is preserved.",
            request_id: "test",
          },
        },
      });
      return;
    }
    const input = route.request().postDataJSON();
    const receipt = {
      ...input,
      run_id: "run_1",
      assignment_id: "a",
      digest: "digest",
      state: "accepted",
      reason: "provider acknowledged direction",
      contract_changed: true,
      created_at: at,
      updated_at: at,
    };
    messages = [receipt];
    await route.fulfill({ json: receipt });
  });
  await page.route("**/api/v1/runs/run_1/controls", async (route) => {
    const input = route.request().postDataJSON();
    if (input.action === "finish") stopRequested = true;
    if (input.action === "undo") stopRequested = false;
    if (input.action === "interrupt") {
      providerState = "interrupted";
      stopRequested = true;
    }
    if (input.action === "continue") {
      providerState = "thread_started";
      attemptID = "attempt_2";
    }
    const receipt = {
      ...input,
      run_id: "run_1",
      mode: input.mode ?? "",
      instructions: input.instructions ?? "",
      digest: "digest",
      state: "accepted",
      reason: "worker acknowledged",
      created_at: at,
      updated_at: at,
    };
    controls = [receipt, ...controls];
    await route.fulfill({ json: receipt });
  });
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
  await page.setViewportSize({ width: 390, height: 700 });
  await page.goto("/work?project=p");
  await expect(page.getByRole("link", { name: "Open active work" })).toBeVisible();
  await page.getByRole("button", { name: "Finish task & stop" }).click();
  await expect(page.getByRole("button", { name: "Undo stop" })).toBeVisible();
  await page.getByRole("button", { name: "Undo stop" }).click();
  await expect(page.getByRole("button", { name: "Finish task & stop" })).toBeEnabled();
  await page.getByRole("tab", { name: "Checks" }).click();
  await expect(page.getByText("zero or unknown selected cases")).toBeVisible();
  await page.getByRole("button", { name: "View retained output" }).click();
  await expect(page.getByText("0 tests")).toBeVisible();
  await page.getByRole("tab", { name: "Activity" }).click();
  const draft = page.getByLabel("Instructions for current turn");
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
  await expect(page.getByLabel("Instructions for current turn")).toHaveValue(
    "Keep the unfinished note while I read.",
  );
  await page.getByRole("button", { name: "Send instructions" }).click();
  await expect(page.getByRole("alert")).toContainText("targeted turn changed");
  await expect(page.getByLabel("Instructions for current turn")).toHaveValue(
    "Keep the unfinished note while I read.",
  );
  await page.getByRole("button", { name: "Send instructions" }).click();
  await expect(page.getByLabel("Instructions for current turn")).toHaveValue("");
  await expect(page.getByRole("region", { name: "Instruction receipts" })).toContainText(
    "accepted",
  );
  await page.getByRole("button", { name: "Interrupt now" }).click();
  await expect(page.getByText(/Interruption requested/)).toBeVisible();
  await page.getByRole("button", { name: "Fresh conversation" }).click();
  await page.getByLabel("Continue with instructions").fill("Use the saved diff");
  await page.getByRole("button", { name: "Continue with instructions" }).click();
  await expect(page.getByText(/Targets the next turn/)).toBeVisible();
  expect(
    await page.evaluate(() => document.documentElement.scrollWidth - window.innerWidth),
  ).toBeLessThanOrEqual(1);
});
