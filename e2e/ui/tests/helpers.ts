import { Page, expect } from "@playwright/test";
import fs from "fs";
import path from "path";

export const examples = path.resolve(__dirname, "../../../docs/examples");
export const testdata = path.resolve(__dirname, "../testdata");

/**
 * Upload a document via the API and navigate to the matrix page.
 * This avoids depending on UI state (upload page vs matrix page).
 */
export async function loadDocument(page: Page, filePath: string) {
  const resp = await page.request.post("/upload", {
    multipart: {
      file: {
        name: path.basename(filePath),
        mimeType: "application/x-yaml",
        buffer: fs.readFileSync(filePath),
      },
    },
  });
  expect(resp.ok(), `upload failed: ${resp.status()} ${await resp.text()}`).toBe(true);
  await page.goto("/");
}
