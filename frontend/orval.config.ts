import { readFile, writeFile } from "node:fs/promises"
import { fileURLToPath } from "node:url"
import { defineConfig } from "orval"

const clientPath = fileURLToPath(new URL("./src/api/generated/auth.ts", import.meta.url))

export default defineConfig({
  auth: {
    input: {
      target: "../backend/docs/swagger.json",
      filters: { tags: ["Personal Access Tokens", "Users"] },
    },
    output: {
      baseUrl: "/api",
      target: clientPath,
      tsconfig: "./tsconfig.app.json",
      client: "react-query",
      httpClient: "fetch",
      override: {
        mutator: {
          path: "./src/api/http.ts",
          name: "apiFetch",
          alias: { "~": fileURLToPath(new URL("./src", import.meta.url)) },
        },
        fetch: { includeHttpResponseReturnType: false, forceSuccessResponse: true },
        query: { version: 5 },
      },
    },
    hooks: {
      afterAllFilesWrite: async () => {
        const client = await readFile(clientPath, "utf8")
        await writeFile(clientPath, client.replaceAll(/from (["'])\.\.\/http(?:\.ts)?\1/g, 'from "~/api/http"'))
      },
    },
  },
})
