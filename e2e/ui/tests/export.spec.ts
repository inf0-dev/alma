import { test, expect } from "@playwright/test";
import path from "path";
import { examples, loadDocument } from "./helpers";

test.describe("export", () => {
  test.beforeEach(async ({ page }) => {
    await loadDocument(page, path.join(examples, "database-selection.yaml"));
    await expect(page).toHaveTitle("Database Selection");
  });

  test("export menu opens on click", async ({ page }) => {
    await page.locator("#export-btn").click();
    await expect(page.locator("#export-menu")).toBeVisible();
  });

  test("export YAML triggers download", async ({ page }) => {
    await page.locator("#export-btn").click();

    const [download] = await Promise.all([
      page.waitForEvent("download"),
      page.locator('.export-option[data-format="yaml"]').click(),
    ]);

    expect(download.suggestedFilename()).toMatch(/\.yaml$/);
  });

  test("export JSON triggers download", async ({ page }) => {
    await page.locator("#export-btn").click();

    const [download] = await Promise.all([
      page.waitForEvent("download"),
      page.locator('.export-option[data-format="json"]').click(),
    ]);

    expect(download.suggestedFilename()).toMatch(/\.json$/);
  });

  test("export Markdown triggers download", async ({ page }) => {
    await page.locator("#export-btn").click();

    const [download] = await Promise.all([
      page.waitForEvent("download"),
      page.locator('.export-option[data-format="md"]').click(),
    ]);

    expect(download.suggestedFilename()).toMatch(/\.md$/);
  });
});
