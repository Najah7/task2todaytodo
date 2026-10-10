import { readFile, writeFile } from "node:fs/promises"
import { fileURLToPath } from "node:url"
import { defineConfig } from "orval"

const clientPath = fileURLToPath(new URL("./src/api/generated/auth.ts", import.meta.url))
const projectsClientPath = fileURLToPath(new URL("./src/api/generated/projects.ts", import.meta.url))
const tasksClientPath = fileURLToPath(new URL("./src/api/generated/tasks.ts", import.meta.url))

async function replaceSharedHttpImport(path: string) {
  const client = await readFile(path, "utf8")
  await writeFile(path, client.replaceAll(/from (["'])\.\.\/http(?:\.ts)?\1/g, 'from "~/api/http"'))
}

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
        await replaceSharedHttpImport(clientPath)
      },
    },
  },
  projects: {
    input: {
      target: "../backend/docs/swagger.json",
      filters: { tags: ["Projects"] },
    },
    output: {
      baseUrl: "/api",
      target: projectsClientPath,
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
        await replaceSharedHttpImport(projectsClientPath)
      },
    },
  },
  tasks: {
    input: {
      target: "../backend/docs/swagger.json",
      filters: { tags: ["Tasks", "ActionItems"] },
    },
    output: {
      baseUrl: "/api",
      target: tasksClientPath,
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
        await replaceSharedHttpImport(tasksClientPath)
      },
    },
  },
})
