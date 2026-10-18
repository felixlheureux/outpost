import { expect, test } from "@playwright/test";

test("Inbox selection, reorder conflict, and partial create stay reviewable", async ({ page }) => {
  const stamp = "2026-10-18T12:00:00Z";
  const issue = (number: number, title: string) => ({
    number,
    url: `https://github.com/owner/repo/issues/${number}`,
    title,
    body: `## Outcome\n${title}\n\n## Acceptance criteria\n- It works.`,
    state: "open",
    source_updated_at: stamp,
    content_digest: `digest-${number}`,
    fetched_at: stamp,
    local_revision: 1,
    requirements: {
      title,
      outcome: title,
      criteria: ["It works."],
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
  });
  const issues = [issue(5, "Queued task"), issue(6, "Inbox task")];
  let order = [5];
  let sha = "sha-one";
  let conflictOnce = true;
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
  await page.route("**/api/v1/projects", (route) =>
    route.fulfill({
      json: {
        projects: [
          {
            id: "project_1",
            name: "owner/repo",
            repository_url: "https://github.com/owner/repo",
            setup_state: "ready",
            setup_stage: "ready",
            created_at: stamp,
          },
        ],
      },
    }),
  );
  await page.route("**/api/v1/projects/project_1/issues", (route) => {
    if (route.request().method() === "POST") {
      const body = route.request().postDataJSON();
      expect(body.title).toBe("New task");
      expect(body.body).toContain("## Outcome\nShip the feature");
      issues.push(issue(7, "New task"));
      return route.fulfill({
        json: {
          id: "issue-op",
          request_key: body.request_key,
          kind: "create",
          state: "confirmed",
          issue_number: 0,
          result_number: 7,
          reason: "",
          created_at: stamp,
        },
      });
    }
    return route.fulfill({
      json: { source: { state: "ready", last_success_at: stamp, last_error: "" }, issues },
    });
  });
  await page.route("**/api/v1/projects/project_1/issues/sync", (route) =>
    route.fulfill({ json: { observed: issues.length } }),
  );
  await page.route("**/api/v1/projects/project_1/issue-operations", (route) =>
    route.fulfill({ json: { operations: [] } }),
  );
  await page.route("**/api/v1/projects/project_1/queue", (route) =>
    route.fulfill({
      json: { sha, issues: order, state: "ready", fetched_at: stamp, last_error: "" },
    }),
  );
  await page.route("**/api/v1/projects/project_1/queue/operations", (route) => {
    if (route.request().method() === "GET") return route.fulfill({ json: { operations: [] } });
    const edit = route.request().postDataJSON();
    expect(edit.expected_sha).toBe(sha);
    const state =
      edit.kind === "move" && conflictOnce
        ? "conflict"
        : edit.issue === 7
          ? "rejected"
          : "confirmed";
    if (edit.kind === "move" && conflictOnce) {
      conflictOnce = false;
      sha = "sha-two";
    } else if (state === "confirmed") {
      if (edit.kind === "append") order = [...order, edit.issue];
      if (edit.kind === "move") order = [edit.issue, ...order.filter((n) => n !== edit.issue)];
      sha = "sha-three";
    }
    return route.fulfill({
      json: {
        id: "queue-op",
        request_key: edit.request_key,
        kind: edit.kind,
        state,
        expected_sha: edit.expected_sha,
        intended: edit,
        current_issues: order,
        reason:
          state === "conflict"
            ? "Manifest changed."
            : state === "rejected"
              ? "Queue write denied."
              : "",
        created_at: stamp,
      },
    });
  });
  const errors: string[] = [];
  page.on("pageerror", (error) => errors.push(error.message));
  await page.goto("/");
  await page.getByRole("tab", { name: "Inbox 1" }).click();
  await page.getByRole("button", { name: /#6 Inbox task/ }).click();
  await expect(page.getByRole("heading", { name: "#6 Inbox task" })).toBeVisible();
  await page.getByRole("button", { name: "More actions" }).click();
  await page.getByRole("menuitem", { name: "Add to end of queue" }).click();
  await expect(page.getByRole("tab", { name: "Queued 2" })).toBeVisible();
  await page.getByRole("tab", { name: "Queued 2" }).click();
  await page.getByRole("button", { name: "More actions" }).click();
  await page.getByRole("menuitem", { name: "Move earlier" }).click();
  await expect(page.getByText(/conflict: Manifest changed/)).toBeVisible();
  await page.getByRole("button", { name: "Apply to latest order" }).click();
  await expect(page.getByRole("button", { name: /#6 Inbox task/ })).toBeVisible();
  await page.getByRole("button", { name: "New task" }).click();
  const dialog = page.getByRole("dialog");
  await dialog.getByLabel("Title").fill("New task");
  await dialog.getByLabel("Desired outcome").fill("Ship the feature");
  await dialog.getByLabel("Acceptance criteria (one per line)").fill("It works.");
  await dialog.getByLabel("Add to the end of the queue after creating").check();
  await dialog.getByRole("button", { name: "Create issue" }).click();
  await expect(dialog.getByText(/Issue #7 was created/)).toBeVisible();
  await expect(dialog.getByText(/It remains in Inbox/)).toBeVisible();
  await expect(dialog.getByText(/Queue: rejected Queue write denied/)).toBeVisible();
  expect(errors).toEqual([]);
});
