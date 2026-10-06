import { defineConfig } from "@playwright/test"

const apiUrl = process.env.E2E_API_URL
const port = process.env.E2E_FRONTEND_PORT
if (!apiUrl || !port) throw new Error("Run pnpm test:e2e to start the isolated backend and database.")
const baseURL = `http://127.0.0.1:${port}`

export default defineConfig({
  testDir: "./e2e",
  fullyParallel: true,
  use: { baseURL, locale: "ja-JP", trace: "retain-on-failure" },
  webServer: {
    command: `pnpm dev --mode e2e --host 127.0.0.1 --port ${port} --strictPort`,
    url: baseURL,
    env: { API_PROXY_TARGET: apiUrl, VITE_API_BASE_URL: "" },
    reuseExistingServer: false,
  },
})
