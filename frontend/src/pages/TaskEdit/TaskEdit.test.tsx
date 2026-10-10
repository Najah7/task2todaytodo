import { afterEach, expect, test, vi } from "vitest"
import { cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react"
import { QueryClient, QueryClientProvider } from "@tanstack/react-query"
import { createMemoryRouter, RouterProvider } from "react-router"
import type { RestProjectOptionsResponse } from "~/api/generated/projects"
import type { RestTaskResponse } from "~/api/generated/tasks"
import { getGetTasksIdQueryKey } from "~/api/generated/tasks"
import { LanguageProviderContext } from "~/features/i18n/languageContext"
import TaskEditPage from "."

afterEach(() => {
  cleanup()
  vi.unstubAllGlobals()
})

const task: RestTaskResponse = {
  action_item_completed_count: 0,
  action_item_count: 0,
  actual_minutes: null,
  assignee_id: "user-1",
  can_update: true,
  created_at: "2026-10-10T00:00:00Z",
  description: "Initial description",
  due_date: null,
  estimate_source: "manual",
  estimated_minutes: null,
  id: "task-1",
  manual_estimated_minutes: null,
  priority: "medium",
  progress: 0,
  project_id: null,
  project_name: null,
  remaining_days: null,
  revision: 3,
  status: "open",
  title: "Original task",
  updated_at: "2026-10-10T00:00:00Z",
  user_id: "user-1",
}

const options: RestProjectOptionsResponse = {
  priorities: [{ label: "Medium", label_jp: "中", value: "medium", weight: 2 }],
  statuses: [],
  types: [],
}

function jsonResponse(value: unknown, status = 200) {
  return new Response(JSON.stringify(value), { status, headers: { "Content-Type": "application/json" } })
}

function renderPage(client = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } })) {
  const router = createMemoryRouter([
    { path: "/tasks/:id/edit", element: <TaskEditPage /> },
    { path: "/tasks", element: <p>Task list</p> },
  ], { initialEntries: ["/tasks/task-1/edit"] })
  render(
    <LanguageProviderContext.Provider value={{ language: "ja", setLanguage: vi.fn() }}>
      <QueryClientProvider client={client}><RouterProvider router={router} /></QueryClientProvider>
    </LanguageProviderContext.Provider>,
  )
  return { client, router }
}

function matchesTask(input: RequestInfo | URL): boolean {
  return new URL(String(input), "http://localhost").pathname === "/api/tasks/task-1"
}

function matchesTaskActionItems(input: RequestInfo | URL): boolean {
  return new URL(String(input), "http://localhost").pathname === "/api/tasks/task-1/action-items"
}

function taskDetails(value: RestTaskResponse) {
  return { task: value }
}

function commonResponse(input: RequestInfo | URL): Response | undefined {
  const url = new URL(String(input), "http://localhost")
  if (url.pathname === "/api/tasks/task-1/action-items") return jsonResponse({ items: [] })
  if (url.pathname === "/api/projects") return jsonResponse({ items: [], next_page_token: "" })
  if (url.pathname === "/api/projects/options") return jsonResponse(options)
  return undefined
}

test("waits for options, captures the ready revision, and submits against that baseline", async () => {
  let resolveOptions!: (response: Response) => void
  const optionsResponse = new Promise<Response>((resolve) => { resolveOptions = resolve })
  let taskReads = 0
  let patchCall: [RequestInfo | URL, RequestInit | undefined] | undefined
  const latestTask = { ...task, title: "Revision four task", revision: 4 }
  const fetchMock = vi.fn(async (input: RequestInfo | URL, init?: RequestInit): Promise<Response> => {
    const shared = commonResponse(input)
    if (shared && new URL(String(input), "http://localhost").pathname !== "/api/projects/options") return shared
    const url = new URL(String(input), "http://localhost")
    if (url.pathname === "/api/projects/options") return optionsResponse
    if (matchesTask(input) && init?.method === "PATCH") {
      patchCall = [input, init]
      return jsonResponse({ ...latestTask, title: "Saved task", revision: 5 })
    }
    if (matchesTask(input)) {
      taskReads += 1
      return jsonResponse(taskDetails(taskReads === 1 ? task : latestTask))
    }
    throw new Error(`Unexpected request: ${url.pathname} ${init?.method ?? "GET"}`)
  })
  vi.stubGlobal("fetch", fetchMock)
  const { client } = renderPage()

  await waitFor(() => expect(fetchMock.mock.calls.some(([input]) => matchesTask(input))).toBe(true))
  await waitFor(() => expect(fetchMock.mock.calls.some(([input]) => new URL(String(input), "http://localhost").pathname === "/api/projects/options")).toBe(true))
  await client.invalidateQueries({ queryKey: getGetTasksIdQueryKey("task-1") })
  await waitFor(() => expect(client.getQueryData<{ task: RestTaskResponse }>(getGetTasksIdQueryKey("task-1"))?.task.revision).toBe(4))

  expect(screen.getByRole("status")).toBeTruthy()
  expect(screen.queryByRole("form")).toBeNull()
  resolveOptions(jsonResponse(options))

  const title = await screen.findByLabelText(/^タスク名/)
  expect((title as HTMLInputElement).value).toBe("Revision four task")
  expect(screen.queryByRole("status")).toBeNull()
  fireEvent.change(title, { target: { value: "Edited using ready revision" } })
  fireEvent.click(screen.getByRole("button", { name: "変更を保存" }))
  await waitFor(() => expect(patchCall).toBeDefined())
  expect(new Headers(patchCall?.[1]?.headers).get("If-Match")).toBe('"4"')
})

test("keeps the ready form and draft when a background task refetch returns 403", async () => {
  let taskReads = 0
  const fetchMock = vi.fn(async (input: RequestInfo | URL): Promise<Response> => {
    const shared = commonResponse(input)
    if (shared) return shared
    if (matchesTaskActionItems(input)) return jsonResponse({ items: [] })
    if (matchesTask(input)) {
      taskReads += 1
      return taskReads === 1 ? jsonResponse(taskDetails(task)) : jsonResponse({ error: { code: "forbidden" } }, 403)
    }
    throw new Error(`Unexpected request: ${new URL(String(input), "http://localhost").pathname}`)
  })
  vi.stubGlobal("fetch", fetchMock)
  const { client } = renderPage()

  const title = await screen.findByLabelText(/^タスク名/)
  fireEvent.change(title, { target: { value: "Draft survives 403" } })
  await client.invalidateQueries({ queryKey: getGetTasksIdQueryKey("task-1") })

  await waitFor(() => expect(taskReads).toBe(2))
  expect(client.getQueryData<{ task: RestTaskResponse }>(getGetTasksIdQueryKey("task-1"))?.task).toEqual(task)
  expect((screen.getByLabelText(/^タスク名/) as HTMLInputElement).value).toBe("Draft survives 403")
  expect(screen.queryByRole("status")).toBeNull()
})

test("keeps draft, confirmation, and cache when explicit reload finds a noneditable task", async () => {
  let taskReads = 0
  const fetchMock = vi.fn(async (input: RequestInfo | URL, init?: RequestInit): Promise<Response> => {
    const shared = commonResponse(input)
    if (shared) return shared
    if (matchesTaskActionItems(input)) return jsonResponse({ items: [] })
    if (matchesTask(input) && init?.method === "PATCH") {
      return jsonResponse({ error: { code: "revision_conflict", details: [{ code: "revision_conflict" }] } }, 409)
    }
    if (matchesTask(input)) {
      taskReads += 1
      return jsonResponse(taskDetails(taskReads === 1 ? task : { ...task, title: "No longer editable", can_update: false, revision: 4 }))
    }
    throw new Error(`Unexpected request: ${new URL(String(input), "http://localhost").pathname} ${init?.method ?? "GET"}`)
  })
  vi.stubGlobal("fetch", fetchMock)
  const { client } = renderPage()

  const title = await screen.findByLabelText(/^タスク名/)
  fireEvent.change(title, { target: { value: "Draft to retain" } })
  fireEvent.click(screen.getByRole("button", { name: "変更を保存" }))
  fireEvent.click(await screen.findByRole("button", { name: "最新の内容を読み込む" }))
  const dialog = await screen.findByRole("alertdialog", { name: "最新の内容を読み込みますか？" })
  fireEvent.click(within(dialog).getByRole("button", { name: "最新の内容を読み込む" }))

  await waitFor(() => expect((within(dialog).getByRole("button", { name: "最新の内容を読み込む" }) as HTMLButtonElement).disabled).toBe(false))
  expect((screen.getByLabelText(/^タスク名/) as HTMLInputElement).value).toBe("Draft to retain")
  expect(screen.getByRole("alertdialog", { name: "最新の内容を読み込みますか？" })).toBeTruthy()
  expect(client.getQueryData<{ task: RestTaskResponse }>(getGetTasksIdQueryKey("task-1"))?.task).toEqual(task)
})

test("retries a failed revision refresh before completing a clean follow-up save", async () => {
  let taskReads = 0
  const actionItem = {
    id: "series-1",
    series_id: "series-1",
    occurrence_date: "2026-10-10",
    title: "Original action item",
    estimated_minutes: 30,
    priority: "medium",
    completed: false,
  }
  const fetchMock = vi.fn(async (input: RequestInfo | URL, init?: RequestInit): Promise<Response> => {
    const shared = commonResponse(input)
    const url = new URL(String(input), "http://localhost")
    if (url.pathname === "/api/tasks/task-1/action-items" && init?.method !== "PATCH") return jsonResponse({ items: [actionItem] })
    if (url.pathname === "/api/tasks/task-1/action-items/series-1" && init?.method === "PATCH") {
      return jsonResponse({ ...actionItem, title: "Saved action item" })
    }
    if (shared) return shared
    if (matchesTask(input)) {
      taskReads += 1
      if (taskReads === 2) return jsonResponse({ error: { code: "temporary_error" } }, 500)
      return jsonResponse(taskDetails(taskReads >= 4 ? { ...task, revision: 4 } : task))
    }
    throw new Error(`Unexpected request: ${url.pathname} ${init?.method ?? "GET"}`)
  })
  vi.stubGlobal("fetch", fetchMock)
  const { router } = renderPage()

  const itemTitle = await screen.findByLabelText("アクションアイテム名 1")
  fireEvent.change(itemTitle, { target: { value: "Saved action item" } })
  fireEvent.click(screen.getByRole("button", { name: "変更を保存" }))
  expect(await screen.findByText("保存した変更の最新状態を確認できませんでした。もう一度お試しください。")).toBeTruthy()
  const readsAfterRefreshFailure = taskReads
  expect(readsAfterRefreshFailure).toBeGreaterThanOrEqual(3)

  fireEvent.click(screen.getByRole("button", { name: "変更を保存" }))
  await waitFor(() => expect(router.state.location.pathname).toBe("/tasks"))
  expect(taskReads).toBeGreaterThan(readsAfterRefreshFailure)
})
