import { expect, test } from "@playwright/test";

test("issue edit conflict keeps the draft across navigation", async ({ page }) => {
  const timestamp = "2026-10-17T12:00:00Z";
  await page.route("**/api/v1/bootstrap", (route) =>
    route.fulfill({
      json: {
        version: "test",
        mode: "development",
        current_run: null,
        capabilities: { repository_setup: true, execution: false },
      },
    }),
  );
  const issue = {
    number: 7,
    url: "https://github.com/owner/repo/issues/7",
    title: "Original title",
    body: "Original body",
    state: "open",
    source_updated_at: timestamp,
    content_digest: "original-digest",
    fetched_at: timestamp,
    local_revision: 1,
    requirements: null,
    parse_error: "",
    stale: false,
  };
  await page.route("**/api/v1/projects", (route) =>
    route.fulfill({
      json: {
        projects: [
          {
            id: "project_7",
            name: "owner/repo",
            repository_url: "https://github.com/owner/repo",
            setup_state: "pending",
            setup_stage: "queue_pending",
            created_at: timestamp,
          },
        ],
      },
    }),
  );
  await page.route("**/api/v1/projects/project_7/issues", (route) =>
    route.fulfill({
      json: {
        source: { state: "ready", last_success_at: timestamp, last_error: "" },
        issues: [issue],
      },
    }),
  );
  await page.route("**/api/v1/projects/project_7/issue-operations", (route) =>
    route.fulfill({ json: { operations: [] } }),
  );
  await page.route("**/api/v1/projects/project_7/issues/sync", (route) =>
    route.fulfill({ json: { observed: 1 } }),
  );
  await page.route("**/api/v1/projects/project_7/queue", (route) =>
    route.fulfill({
      json: { sha: "sha-1", issues: [7], state: "ready", fetched_at: timestamp, last_error: "" },
    }),
  );
  await page.route("**/api/v1/projects/project_7/queue/operations", (route) =>
    route.fulfill({ json: { operations: [] } }),
  );
  await page.route("**/api/v1/projects/project_7/issues/7", async (route) => {
    const body = route.request().postDataJSON();
    expect(body.title).toBe("Revised title");
    expect(body.body).toBe("Unsent changes");
    expect(body.expected_digest).toBe("original-digest");
    expect(body.expected_revision).toBe(1);
    await route.fulfill({
      json: {
        id: "operation-1",
        request_key: body.request_key,
        kind: "edit",
        state: "conflict",
        issue_number: 7,
        result_number: 0,
        reason: "Issue changed on GitHub.",
        created_at: timestamp,
      },
    });
  });
  await page.goto("/");
  await expect(page.getByRole("button", { name: /#7 Original title/ })).toBeVisible();
  await page.getByRole("button", { name: /#7 Original title/ }).click();
  await page.getByRole("button", { name: "Edit task" }).click();
  const dialog = page.getByRole("dialog");
  await dialog.getByLabel("Title").fill("Revised title");
  await dialog.getByLabel("Specification").fill("Unsent changes");
  await dialog.getByRole("button", { name: "Save issue" }).click();
  await expect(dialog.getByText(/conflict: Issue changed/)).toBeVisible();
  await dialog.locator("form").getByRole("button", { name: "Close" }).click();
  await page.getByRole("link", { name: "Work", exact: true }).click();
  await page.getByRole("link", { name: "Queue", exact: true }).click();
  await page.getByRole("button", { name: "Edit task" }).click();
  await expect(dialog.getByLabel("Title")).toHaveValue("Revised title");
  await expect(dialog.getByLabel("Specification")).toHaveValue("Unsent changes");
});
