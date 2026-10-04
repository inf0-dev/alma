import { test, expect } from "@playwright/test";
import path from "path";
import { examples, loadDocument } from "./helpers";

test.describe("decision workflow", () => {
  test.beforeEach(async ({ page }) => {
    await loadDocument(page, path.join(examples, "database-selection.yaml"));
    await expect(page).toHaveTitle("Database Selection");
  });

  test("no decision initially", async ({ page }) => {
    await expect(page.locator("#decision-empty")).toBeVisible();
    await expect(page.locator("#decision-made")).not.toBeVisible();
  });

  test("finalize a decision with rationale and attendees", async ({ page }) => {
    // Select option in list to view its detail panel
    await page.locator('[data-option-id="opt-postgres"]').click();

    // The pick area should become visible for a possible option
    const pickArea = page.locator('.pick-area[data-option="opt-postgres"]');
    await expect(pickArea).toBeVisible();

    await pickArea.locator(".pick-btn").click();

    // The form should now be visible
    const pickForm = pickArea.locator(".pick-form");
    await expect(pickForm).toBeVisible();

    await pickArea.locator(".pick-present").fill("Alice, Bob");
    await pickArea.locator(".pick-rationale").fill("Best overall fit");
    await pickArea.locator(".pick-confirm").click();

    await expect(page.locator("#decision-made")).toBeVisible();
    await expect(page.locator("#decision-title")).toContainText("PostgreSQL");
    await expect(page.locator("#decision-empty")).not.toBeVisible();
  });

  test("undo decision restores empty state", async ({ page }) => {
    // Make a decision first
    await page.locator('[data-option-id="opt-postgres"]').click();
    const pickArea = page.locator('.pick-area[data-option="opt-postgres"]');
    await expect(pickArea).toBeVisible();
    await pickArea.locator(".pick-btn").click();
    await pickArea.locator(".pick-present").fill("Alice");
    await pickArea.locator(".pick-confirm").click();
    await expect(page.locator("#decision-made")).toBeVisible();

    // Undo
    await page.locator("#decision-undo").click();

    // Confirm if modal appears
    const modal = page.locator("#confirm-modal");
    if (await modal.isVisible()) {
      await page.locator("#confirm-modal-confirm").click();
    }

    await expect(page.locator("#decision-empty")).toBeVisible();
    await expect(page.locator("#decision-made")).not.toBeVisible();
  });

  test("re-decide moves previous decision to history", async ({ page }) => {
    // First decision
    await page.locator('[data-option-id="opt-postgres"]').click();
    const pgPick = page.locator('.pick-area[data-option="opt-postgres"]');
    await expect(pgPick).toBeVisible();
    await pgPick.locator(".pick-btn").click();
    await pgPick.locator(".pick-present").fill("Alice");
    await pgPick.locator(".pick-confirm").click();
    await expect(page.locator("#decision-title")).toContainText("PostgreSQL");

    // Second decision
    await page.locator('[data-option-id="opt-mongo"]').click();
    const mongoPick = page.locator('.pick-area[data-option="opt-mongo"]');
    await expect(mongoPick).toBeVisible();
    await mongoPick.locator(".pick-btn").click();
    await mongoPick.locator(".pick-present").fill("Bob");
    await mongoPick.locator(".pick-confirm").click();

    // Confirm replacement if modal appears
    const modal = page.locator("#confirm-modal");
    if (await modal.isVisible()) {
      await page.locator("#confirm-modal-confirm").click();
    }

    await expect(page.locator("#decision-title")).toContainText("MongoDB");
    await expect(page.locator("#history-section")).toBeVisible();
  });

  test("state change blocking picked option shows confirmation", async ({
    page,
  }) => {
    // Pick SQLite
    await page.locator('[data-option-id="opt-sqlite"]').click();
    const pickArea = page.locator('.pick-area[data-option="opt-sqlite"]');
    await expect(pickArea).toBeVisible();
    await pickArea.locator(".pick-btn").click();
    await pickArea.locator(".pick-present").fill("Alice");
    await pickArea.locator(".pick-confirm").click();
    await expect(page.locator("#decision-title")).toContainText("SQLite");

    // Select "large" scale (this should block SQLite)
    await page
      .locator('button[data-item="q-scale"][data-answer="large"]')
      .click();

    // Confirmation modal should appear
    var modal = page.locator("#confirm-modal");
    await expect(modal).toBeVisible();

    // Canceling should mean decision should remain, choice should revert
    await page.locator("#confirm-modal-cancel").click();
    await expect(modal).not.toBeVisible();
    await expect(page.locator("#decision-title")).toContainText("SQLite");
    await expect(
      page.locator('button[data-item="q-scale"][data-answer="large"]'),
    ).toHaveAttribute("aria-pressed", "false");

    // Select "large" scale again, but this time accept change
    await page
      .locator('button[data-item="q-scale"][data-answer="large"]')
      .click();
    modal = page.locator("#confirm-modal");
    await expect(modal).toBeVisible();
    await page.locator("#confirm-modal-confirm").click();
    await expect(modal).not.toBeVisible();
    await expect(page.locator("#decision-empty")).toBeVisible();
    await expect(page.locator("#decision-made")).not.toBeVisible();
    await expect(
      page.locator('button[data-item="q-scale"][data-answer="large"]'),
    ).toHaveAttribute("aria-pressed", "true");
  });

  test("confirming state change that blocks picked option clears decision", async ({
    page,
  }) => {
    // Pick SQLite
    await page.locator('[data-option-id="opt-sqlite"]').click();
    const pickArea = page.locator('.pick-area[data-option="opt-sqlite"]');
    await expect(pickArea).toBeVisible();
    await pickArea.locator(".pick-btn").click();
    await pickArea.locator(".pick-present").fill("Alice");
    await pickArea.locator(".pick-confirm").click();
    await expect(page.locator("#decision-title")).toContainText("SQLite");

    // Select "large" scale — triggers confirmation
    await page
      .locator('button[data-item="q-scale"][data-answer="large"]')
      .click();

    const modal = page.locator("#confirm-modal");
    await expect(modal).toBeVisible();

    // Confirm — decision should be cleared, choice applied
    await page.locator("#confirm-modal-confirm").click();
    await expect(modal).not.toBeVisible();
    await expect(page.locator("#decision-empty")).toBeVisible();
    await expect(
      page.locator('button[data-item="q-scale"][data-answer="large"]'),
    ).toHaveAttribute("aria-pressed", "true");
  });

  test("pick button hidden for blocked options", async ({ page }) => {
    // Block SQLite by selecting large data volume
    await page
      .locator('button[data-item="q-scale"][data-answer="large"]')
      .click();

    await page.locator('[data-option-id="opt-sqlite"]').click();

    const pickArea = page.locator('.pick-area[data-option="opt-sqlite"]');
    await expect(pickArea).not.toBeVisible();
  });
});
