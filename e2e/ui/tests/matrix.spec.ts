import { test, expect } from "@playwright/test";
import path from "path";
import { examples, loadDocument } from "./helpers";

test.describe("matrix interactions", () => {
  test.beforeEach(async ({ page }) => {
    await loadDocument(page, path.join(examples, "database-selection.yaml"));
    await expect(page).toHaveTitle("Database Selection");
  });

  test("requirement toggles update aria-pressed", async ({ page }) => {
    const hardReq = page.locator('.req[data-id="req-acid"]');

    await hardReq.click();
    await expect(hardReq).toHaveAttribute("aria-pressed", "true");

    await hardReq.click();
    await expect(hardReq).toHaveAttribute("aria-pressed", "false");
  });

  test("choice item selection", async ({ page }) => {
    const largeBtn = page.locator(
      'button[data-item="q-scale"][data-answer="large"]',
    );
    const smallBtn = page.locator(
      'button[data-item="q-scale"][data-answer="small"]',
    );

    await largeBtn.click();
    await expect(largeBtn).toHaveAttribute("aria-pressed", "true");
    await expect(smallBtn).toHaveAttribute("aria-pressed", "false");
  });

  test("choice selection triggers block on option", async ({ page }) => {
    const sqliteItem = page.locator('[data-option-id="opt-sqlite"]');
    await expect(sqliteItem).not.toHaveClass(/blocked/);

    await page
      .locator('button[data-item="q-scale"][data-answer="large"]')
      .click();

    await expect(sqliteItem).toHaveClass(/blocked/);
  });

  test("number item accepts input", async ({ page }) => {
    const numberInput = page.locator("#input-q-replicas");
    await expect(numberInput).toBeVisible();
    await numberInput.fill("3");
    await expect(numberInput).toHaveValue("3");
  });

  test("text item accepts input", async ({ page }) => {
    const textInput = page.locator("#input-q-notes");
    await expect(textInput).toBeVisible();
    await textInput.fill("Must support multi-region");
    await expect(textInput).toHaveValue("Must support multi-region");
  });

  test("option detail panel switches on click", async ({ page }) => {
    const mongoItem = page.locator('[data-option-id="opt-mongo"]');
    await mongoItem.click();

    const mongoPanel = page.locator("#option-opt-mongo");
    await expect(mongoPanel).toHaveClass(/active/);

    const pgPanel = page.locator("#option-opt-postgres");
    await expect(pgPanel).not.toHaveClass(/active/);
  });

  test("option shows pros and cons", async ({ page }) => {
    const pgPanel = page.locator("#option-opt-postgres");
    await expect(pgPanel.locator(".pros li")).not.toHaveCount(0);
    await expect(pgPanel.locator(".cons li")).not.toHaveCount(0);
  });

  test("show_when item is hidden initially", async ({ page }) => {
    const partitionItem = page.locator(".item[data-show-when]");
    await expect(partitionItem).toBeHidden();
  });

  test("show_when item appears when condition is met", async ({ page }) => {
    const partitionItem = page.locator(".item[data-show-when]");
    await expect(partitionItem).toBeHidden();

    // Select "large" scale to satisfy show_when: [["q-scale.large"]]
    await page
      .locator('button[data-item="q-scale"][data-answer="large"]')
      .click();

    await expect(partitionItem).toBeVisible();
  });

  test("show_when item hides again and answer clears when condition unmet", async ({
    page,
  }) => {
    const partitionItem = page.locator(".item[data-show-when]");

    // Show the item
    await page
      .locator('button[data-item="q-scale"][data-answer="large"]')
      .click();
    await expect(partitionItem).toBeVisible();

    // Select an answer in the conditional item
    const rangeBtn = page.locator(
      'button[data-item="q-partition"][data-answer="range"]',
    );
    await rangeBtn.click();
    await expect(rangeBtn).toHaveAttribute("aria-pressed", "true");

    // Change scale to small — partition should hide
    await page
      .locator('button[data-item="q-scale"][data-answer="small"]')
      .click();
    await expect(partitionItem).toBeHidden();

    // Re-show: answer should have been cleared
    await page
      .locator('button[data-item="q-scale"][data-answer="large"]')
      .click();
    await expect(partitionItem).toBeVisible();
    await expect(rangeBtn).toHaveAttribute("aria-pressed", "false");
  });
});
