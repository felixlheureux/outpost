import { expect, test } from "@playwright/test";

const stamp = "2026-10-18T12:00:00Z";
const longWord = "longunbrokenrepositoryorissuetext".repeat(8);
const projectName = `owner/${longWord}`;
const title = `Keep the queue readable when a task contains ${longWord}`;

test("Queue content stays inside its panels on desktop and phone", async ({ page }) => {
  test.setTimeout(90000);
  let currentRun: null | { id: string; project_id: string; state: string; liveness: string } = null;
  await page.route("**/api/v1/bootstrap", (route) =>
    route.fulfill({
      json: {
        version: "71eea935e70a3a018bdd0d37ac77c50a68836e1f",
        mode: "development",
        current_run: currentRun,
        capabilities: { repository_setup: true, execution: false },
      },
    }),
  );
  await page.route("**/api/v1/projects", (route) =>
    route.fulfill({
      json: {
        projects: [
          {
            id: "project_1",
            name: projectName,
            repository_url: "https://github.com/owner/repo",
            setup_state: "ready",
            setup_stage: "ready",
            created_at: stamp,
          },
        ],
      },
    }),
  );
  await page.route("**/api/v1/projects/project_1/issues", (route) =>
    route.fulfill({
      json: {
        source: { state: "ready", last_success_at: stamp, last_error: "" },
        issues: [
          {
            number: 7,
            url: "https://github.com/owner/repo/issues/7",
            title,
            body: `## Outcome\n${longWord}`,
            state: "open",
            source_updated_at: stamp,
            content_digest: "digest-7",
            fetched_at: stamp,
            local_revision: 1,
            requirements: {
              title,
              outcome: longWord,
              criteria: [longWord],
              notes: "",
              settings: {
                kind: "implementation",
                depends_on: [],
                hold_reason: null,
                check_profile: "inherit",
                delivery: "inherit",
                review: "inherit",
              },
              has_settings: false,
            },
            parse_error: "",
            stale: false,
          },
        ],
      },
    }),
  );
  await page.route("**/api/v1/projects/project_1/issue-operations", (route) =>
    route.fulfill({ json: { operations: [] } }),
  );
  await page.route("**/api/v1/projects/project_1/queue", (route) =>
    route.fulfill({
      json: {
        sha: longWord,
        issues: [7],
        state: "ready",
        fetched_at: stamp,
        last_error: "",
      },
    }),
  );
  await page.route("**/api/v1/projects/project_1/queue/operations", (route) =>
    route.fulfill({ json: { operations: [] } }),
  );
  await page.route("**/api/v1/agent/capabilities?project=project_1", (route) =>
    route.fulfill({
      json: {
        version: "codex-cli 0.163.0",
        models: [
          {
            model: "fixture-a",
            displayName: "Fixture A",
            hidden: false,
            isDefault: true,
            defaultReasoningEffort: "low",
            supportedReasoningEfforts: [{ reasoningEffort: "low" }, { reasoningEffort: "medium" }],
          },
          {
            model: "fixture-b",
            displayName: "Fixture B",
            hidden: false,
            isDefault: false,
            defaultReasoningEffort: "high",
            supportedReasoningEfforts: [{ reasoningEffort: "high" }],
          },
        ],
        resume: true,
        steer: true,
        interrupt: true,
        structured_results: true,
        permission_controls: true,
      },
    }),
  );
  await page.route("**/api/v1/runs/run-1/work", (route) =>
    route.fulfill({
      json: {
        run_id: "run-1",
        project_id: "project_1",
        run_state: "running",
        assignment_id: "assignment",
        attempt_id: "attempt",
        attempt_phase: "implementing",
        attempt_outcome: "running",
        provider_state: "turn_started",
        thread_id: "thread",
        turn_id: "turn",
        turn_open: true,
        issue_number: 7,
        snapshot: { issue_number: 7, title, body: longWord },
        model: "fixture-a",
        reasoning: "medium",
        sandbox: "workspace-write",
        approval_policy: "on-request",
        successful_count: 0,
        task_limit: 1,
        stop_requested: false,
        schedule_outcome: "active",
        schedule_reason: "",
        messages: [],
        controls: [],
        checks: [],
      },
    }),
  );
  for (const width of [1440, 1024, 820, 768, 390, 320]) {
    await page.setViewportSize({ width, height: 900 });
    await page.goto("/");
    if (width <= 900 && (await page.locator(".queue-grid.show-detail").count())) {
      await page.getByRole("button", { name: "Back to tasks" }).click();
    }
    await page.getByRole("button", { name: /#7 Keep the queue readable/ }).click();
    if (width <= 900) {
      await expect(page.locator(".queue-list")).toBeHidden();
      await expect(page.getByRole("heading", { name: `#7 ${title}` })).toBeFocused();
    }
    const footerEdges = await page.locator(".queue-inspector").evaluate((inspector) => {
      const panel = inspector.getBoundingClientRect();
      const footer = inspector.querySelector(".queue-inspector-actions")?.getBoundingClientRect();
      return footer
        ? { right: panel.right - footer.right, bottom: panel.bottom - footer.bottom }
        : null;
    });
    expect(footerEdges, `inspector footer at ${width}px`).not.toBeNull();
    expect(footerEdges?.right).toBeLessThanOrEqual(2);
    expect(footerEdges?.bottom).toBeLessThanOrEqual(2);
    if (width === 1440 || width === 390)
      await page.screenshot({ path: `/tmp/outpost-inspector-${width}.png`, fullPage: true });
    await expect(page.getByLabel("Model")).toHaveCount(0);
    await page.locator(".queue-source-details summary").click();
    const overflow = await page.evaluate(() => {
      const selectors = [
        "html",
        ".app-shell",
        ".sidebar",
        ".main-column",
        "#main-content",
        ".page",
        ".project-setup-alert",
        ".queue-grid",
        ".queue-list",
        ".queue-inspector",
        ".queue-source-details",
      ];
      return selectors.flatMap((selector) => {
        const element = document.querySelector(selector);
        if (!element) return [];
        return element.scrollWidth > element.clientWidth + 1
          ? [`${selector}: ${element.scrollWidth} > ${element.clientWidth}`]
          : [];
      });
    });
    expect(overflow, `horizontal overflow at ${width}px`).toEqual([]);
    await page.getByRole("button", { name: "More actions" }).click();
    await expect(page.getByRole("menuitem", { name: "Open on GitHub" })).toBeVisible();
    await expect(page.getByRole("menuitem", { name: "Move earlier" })).toBeDisabled();
    const menuOverflow = await page
      .locator('[data-slot="dropdown-menu-content"]')
      .evaluate((element) => ({
        left: element.getBoundingClientRect().left,
        right: element.getBoundingClientRect().right - window.innerWidth,
      }));
    expect(menuOverflow.left, `menu left edge at ${width}px`).toBeGreaterThanOrEqual(-1);
    expect(menuOverflow.right, `menu right edge at ${width}px`).toBeLessThanOrEqual(1);
    await page.keyboard.press("Escape");
    if (width <= 900) {
      await page.getByRole("button", { name: "Back to tasks" }).click();
      await expect(page.getByRole("button", { name: /#7 Keep the queue readable/ })).toBeFocused();
      await expect(page.locator(".queue-list")).toBeVisible();
      await page.getByRole("button", { name: /#7 Keep the queue readable/ }).click();
    }
    await page.getByRole("button", { name: "Edit task" }).click();
    const dialogOverflow = await page.locator(".issue-editor-dialog").evaluate((element) => ({
      content: element.scrollWidth - element.clientWidth,
      right: element.getBoundingClientRect().right - window.innerWidth,
    }));
    expect(dialogOverflow.content, `dialog content overflow at ${width}px`).toBeLessThanOrEqual(1);
    expect(dialogOverflow.right, `dialog viewport overflow at ${width}px`).toBeLessThanOrEqual(1);
    await page.keyboard.press("Escape");
    await page.getByRole("link", { name: "Start work" }).click();
    await expect(page).toHaveURL(/\/work/);
    await expect(page.getByLabel("Model")).toBeEnabled();
    await expect(page.getByLabel("Reasoning")).toBeEnabled();
    await expect(page.getByLabel("Reasoning")).toHaveText("medium");
    await expect(
      page.getByLabel("Close the GitHub issue after the completed result comment"),
    ).toBeChecked();
    const workOverflow = await page.evaluate(() =>
      ["html", ".page", ".start-panel"].flatMap((selector) => {
        const element = document.querySelector(selector);
        return element && element.scrollWidth > element.clientWidth + 1 ? [selector] : [];
      }),
    );
    expect(workOverflow, `Work overflow at ${width}px`).toEqual([]);
    if (width === 1440 || width === 390)
      await page.screenshot({ path: `/tmp/outpost-work-${width}.png`, fullPage: true });
  }
  await page.getByLabel("Close the GitHub issue after the completed result comment").uncheck();
  await page.reload();
  await expect(
    page.getByLabel("Close the GitHub issue after the completed result comment"),
  ).not.toBeChecked();
  currentRun = { id: "run-1", project_id: "project_1", state: "running", liveness: "active" };
  await page.goto("/work?project=project_1");
  await expect(page.getByRole("heading", { name: `#7 ${title}` })).toBeVisible();
  await expect(page.getByLabel("Model")).toHaveCount(0);
  const activeOverflow = await page.evaluate(() =>
    ["html", ".page", ".activity-work"].filter((selector) => {
      const element = document.querySelector(selector);
      return element && element.scrollWidth > element.clientWidth + 1;
    }),
  );
  expect(activeOverflow).toEqual([]);
  await page.screenshot({ path: "/tmp/outpost-active-work-320.png", fullPage: true });
  await page.setViewportSize({ width: 1440, height: 900 });
  await page.screenshot({ path: "/tmp/outpost-active-work-1440.png", fullPage: true });
});
