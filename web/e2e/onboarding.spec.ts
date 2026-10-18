import { expect, test } from "@playwright/test";

test("URL onboarding and second repository keep separate browsing context", async ({ page }) => {
  const stamp = "2026-10-18T12:00:00Z";
  const projects: Array<Record<string, unknown>> = [];
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
  await page.route("**/api/v1/projects", (route) => {
    if (route.request().method() === "GET") return route.fulfill({ json: { projects } });
    const repositoryURL = route.request().postDataJSON().repository_url as string;
    const name = repositoryURL.replace("https://github.com/", "");
    const project = {
      id: `project_${projects.length + 1}`,
      name,
      repository_url: repositoryURL,
      setup_state: "ready",
      setup_stage: "ready",
      created_at: stamp,
    };
    projects.push(project);
    return route.fulfill({ json: project });
  });
  await page.route(/\/api\/v1\/projects\/project_\d+\/issues$/, (route) =>
    route.fulfill({
      json: { source: { state: "ready", last_success_at: stamp, last_error: "" }, issues: [] },
    }),
  );
  await page.route(/\/api\/v1\/projects\/project_\d+\/queue$/, (route) =>
    route.fulfill({
      json: { sha: "empty-sha", issues: [], state: "ready", fetched_at: stamp, last_error: "" },
    }),
  );
  await page.route(/\/api\/v1\/projects\/project_\d+\/issue-operations$/, (route) =>
    route.fulfill({ json: { operations: [] } }),
  );
  await page.route(/\/api\/v1\/projects\/project_\d+\/queue\/operations$/, (route) =>
    route.fulfill({ json: { operations: [] } }),
  );
  await page.goto("/");
  await page.getByLabel("GitHub repository URL").fill("https://github.com/owner/alpha");
  await page.getByRole("button", { name: "Add", exact: true }).click();
  await expect(page.getByRole("button", { name: "Choose repository" })).toContainText(
    "owner/alpha",
  );
  await page.getByRole("button", { name: "Choose repository" }).click();
  await page.getByRole("menuitem", { name: "Add repository" }).click();
  const dialog = page.getByRole("dialog");
  await dialog.getByLabel("GitHub repository").fill("https://github.com/owner/beta");
  await dialog.getByRole("button", { name: "Add repository" }).click();
  await expect(page.getByRole("button", { name: "Choose repository" })).toContainText("owner/beta");
  await page.getByRole("button", { name: "Choose repository" }).click();
  await page.getByRole("menuitem", { name: "owner/alpha" }).click();
  await expect(page.getByRole("button", { name: "Choose repository" })).toContainText(
    "owner/alpha",
  );
  expect(projects).toHaveLength(2);
});
