import { test, expect } from "@playwright/test";
import path from "path";
import { examples, loadDocument } from "./helpers";

test.describe("navigation", () => {
  test("unknown path returns 404", async ({ page }) => {
    const resp = await page.goto("/nonexistent");
    expect(resp?.status()).toBe(404);
  });

  test("theme toggle switches data-theme attribute", async ({ page }) => {
    await loadDocument(page, path.join(examples, "database-selection.yaml"));

    const html = page.locator("html");
    const toggle = page.locator("#theme-toggle");

    await toggle.click();
    const themeAfterFirst = await html.getAttribute("data-theme");
    expect(themeAfterFirst).toMatch(/^(light|dark)$/);

    await toggle.click();
    const themeAfterSecond = await html.getAttribute("data-theme");
    expect(themeAfterSecond).not.toBe(themeAfterFirst);
  });
});
