import { expect, test } from "@playwright/test";

test("embedded SPA, honest setup boundary, preserved draft and mobile navigation", async ({
  page,
  request,
}) => {
  const errors: string[] = [];
  page.on("pageerror", (error) => errors.push(error.message));
  await page.goto("/");
  await expect(page.getByRole("heading", { name: "Bring your next project." })).toBeVisible();
  await page.getByRole("button", { name: "Add repository", exact: true }).last().click();
  const dialog = page.getByRole("dialog");
  const input = dialog.getByLabel("GitHub repository");
  await input.fill("https://github.com/felixlheureux/outpost");
  await dialog.getByRole("button", { name: "Add repository", exact: true }).click();
  await expect(dialog.getByRole("alert")).toContainText("Your URL has been kept");
  await dialog.getByRole("button", { name: "Cancel" }).click();
  await page.getByRole("link", { name: "Work", exact: true }).click();
  await expect(page.getByRole("heading", { name: "No active run" })).toBeVisible();
  await page.getByRole("button", { name: "Add repository", exact: true }).click();
  await page.getByRole("menuitem", { name: "Add repository" }).click();
  await expect(input).toHaveValue("https://github.com/felixlheureux/outpost");
  await page.keyboard.press("Escape");
  await page.getByRole("link", { name: "History", exact: true }).click();
  await page.reload();
  await expect(page.getByRole("heading", { name: "No attempts yet" })).toBeVisible();
  await page.setViewportSize({ width: 390, height: 844 });
  await page.getByRole("link", { name: "Queue", exact: true }).click();
  await expect(page.getByRole("heading", { name: "Bring your next project." })).toBeVisible();
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
  for (const path of ["/api/v1/typo", "/assets/missing.js"]) {
    const response = await request.get(path, { headers: { Accept: "text/html" } });
    expect(response.status()).toBe(404);
    expect(response.headers()["content-type"]).toContain("application/json");
  }
  expect(errors).toEqual([]);
});
