import { expect, test } from "@playwright/test";

const stamp = "2026-10-18T12:00:00Z";
const longWord = "longunbrokenrepositoryorissuetext".repeat(8);
const projectName = `owner/${longWord}`;
const title = `Keep the queue readable when a task contains ${longWord}`;

test("Queue content stays inside its panels on desktop and phone", async ({ page }) => {
  await page.route("**/api/v1/bootstrap", (route) =>
    route.fulfill({
      json: {
        version: "71eea935e70a3a018bdd0d37ac77c50a68836e1f",
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
  for (const width of [1440, 768, 390, 320]) {
    await page.setViewportSize({ width, height: 900 });
    await page.goto("/");
    await page.getByRole("button", { name: /#7 Keep the queue readable/ }).click();
    await page.getByRole("button", { name: "Review start options" }).click();
    await page.locator(".queue-source-details summary").click();
    const overflow = await page.evaluate(() => {
      const selectors = [
        "html",
        ".app-shell",
        ".sidebar",
        ".main-column",
        "#main-content",
        ".page",
        ".project-list article",
        ".queue-grid",
        ".queue-list",
        ".queue-inspector",
        ".start-panel",
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
    if (width === 1440 || width === 390)
      await page.screenshot({ path: `/tmp/outpost-layout-${width}.png`, fullPage: true });
    expect(overflow, `horizontal overflow at ${width}px`).toEqual([]);
    await page.getByRole("button", { name: "Edit task" }).click();
    const dialogOverflow = await page.locator(".issue-editor-dialog").evaluate((element) => ({
      content: element.scrollWidth - element.clientWidth,
      right: element.getBoundingClientRect().right - window.innerWidth,
    }));
    expect(dialogOverflow.content, `dialog content overflow at ${width}px`).toBeLessThanOrEqual(1);
    expect(dialogOverflow.right, `dialog viewport overflow at ${width}px`).toBeLessThanOrEqual(1);
  }
});
