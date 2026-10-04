import { defineConfig } from "@playwright/test";

export default defineConfig({
  testDir: "./tests",
  webServer: {
    command: "just serve",
    port: 8080,
    reuseExistingServer: !process.env.CI,
  },
});
