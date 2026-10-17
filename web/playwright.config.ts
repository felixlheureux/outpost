import { defineConfig } from "@playwright/test";

export default defineConfig({
  testDir: "e2e",
  fullyParallel: false,
  workers: 1,
  retries: 0,
  reporter: "list",
  use: {
    baseURL: "http://127.0.0.1:17331",
    viewport: { width: 1440, height: 1000 },
    launchOptions: process.env.OUTPOST_TEST_CHROMIUM
      ? { executablePath: process.env.OUTPOST_TEST_CHROMIUM, args: ["--no-sandbox"] }
      : undefined,
  },
  webServer: {
    command: "python3 ../scripts/smoke-server.py",
    url: "http://127.0.0.1:17331/healthz",
    reuseExistingServer: false,
  },
});
