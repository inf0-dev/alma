import { defineConfig } from "@playwright/test";

export default defineConfig({
  testDir: "./tests",
  workers: 1,
  webServer: {
    command: "just serve",
    port: 8080,
    reuseExistingServer: false,
  },
});
