import { test, expect } from "@playwright/test";

test("upload page loads on root", async ({ page }) => {
  await page.goto("/");
  await expect(page).toHaveTitle(/alma/);
});
