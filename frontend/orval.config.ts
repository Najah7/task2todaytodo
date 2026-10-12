import { readFile, writeFile } from "node:fs/promises"
import { fileURLToPath } from "node:url"
import { defineConfig } from "orval"

const clientPath = fileURLToPath(new URL("./src/api/generated/auth.ts", import.meta.url))
const projectsClientPath = fileURLToPath(new URL("./src/api/generated/projects.ts", import.meta.url))
const tasksClientPath = fileURLToPath(new URL("./src/api/generated/tasks.ts", import.meta.url))
const schedulesClientPath = fileURLToPath(new URL("./src/api/generated/schedules.ts", import.meta.url))
const inboxClientPath = fileURLToPath(new URL("./src/api/generated/inbox.ts", import.meta.url))
const allocationsClientPath = fileURLToPath(new URL("./src/api/generated/allocations.ts", import.meta.url))
const todoClientPath = fileURLToPath(new URL("./src/api/generated/todo.ts", import.meta.url))
const metricsClientPath = fileURLToPath(new URL("./src/api/generated/metrics.ts", import.meta.url))

async function replaceSharedHttpImport(path: string) {
  const client = await readFile(path, "utf8")
  await writeFile(path, client.replaceAll(/from (["'])\.\.\/http(?:\.ts)?\1/g, 'from "~/api/http"'))
}

export default defineConfig({
  auth: {
    input: {
      target: "../backend/docs/swagger.json",
      filters: { tags: ["Personal Access Tokens", "Users", "Authorization", "Project Invitations"] },
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
  schedules: {
    input: {
      target: "../backend/docs/swagger.json",
      filters: { tags: ["Schedules"] },
    },
    output: {
      baseUrl: "/api",
      target: schedulesClientPath,
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
        await replaceSharedHttpImport(schedulesClientPath)
      },
    },
  },
  inbox: {
    input: {
      target: "../backend/docs/swagger.json",
      filters: { tags: ["Inbox"] },
    },
    output: {
      baseUrl: "/api",
      target: inboxClientPath,
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
        await replaceSharedHttpImport(inboxClientPath)
      },
    },
  },
  allocations: {
    input: {
      target: "../backend/docs/swagger.json",
      filters: { tags: ["Allocations"] },
    },
    output: {
      baseUrl: "/api",
      target: allocationsClientPath,
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
        await replaceSharedHttpImport(allocationsClientPath)
      },
    },
  },
  todo: {
    input: {
      target: "../backend/docs/swagger.json",
      filters: { tags: ["Todo"] },
    },
    output: {
      baseUrl: "/api",
      target: todoClientPath,
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
        await replaceSharedHttpImport(todoClientPath)
      },
    },
  },
  metrics: {
    input: {
      target: "../backend/docs/swagger.json",
      filters: { tags: ["Metrics"] },
    },
    output: {
      baseUrl: "/api",
      target: metricsClientPath,
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
        await replaceSharedHttpImport(metricsClientPath)
      },
    },
  },
})
