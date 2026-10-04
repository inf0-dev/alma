import { test, expect } from "@playwright/test";
import path from "path";
import { examples, testdata, loadDocument } from "./helpers";

test.describe("upload page", () => {
  test("upload valid document via API renders matrix", async ({ page }) => {
    await loadDocument(page, path.join(examples, "database-selection.yaml"));
    await expect(page).toHaveTitle("Database Selection");
    await expect(page.locator("h1")).toContainText("Database Selection");
    await expect(page.locator(".req")).toHaveCount(4);
  });

  test("upload invalid file returns error", async ({ page }) => {
    const resp = await page.request.post("/upload", {
      multipart: {
        file: {
          name: "invalid.yaml",
          mimeType: "application/x-yaml",
          buffer: Buffer.from("not: a: valid: alma: document\n"),
        },
      },
    });
    expect(resp.ok()).toBe(false);
    expect(resp.status()).toBe(400);
  });

  test("upload replaces existing document", async ({ page }) => {
    await loadDocument(page, path.join(examples, "database-selection.yaml"));
    await expect(page).toHaveTitle("Database Selection");

    // Load a different document via the header load button
    const loadInput = page.locator("#load-file-input");
    await loadInput.setInputFiles(path.join(examples, "auth-strategy.yaml"));
    await expect(page).toHaveTitle("Authentication Strategy");
  });
});
