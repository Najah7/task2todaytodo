import { act, cleanup, render, screen } from "@testing-library/react"
import { QueryClient, QueryClientProvider } from "@tanstack/react-query"
import { useContext } from "react"
import { afterEach, expect, test, vi } from "vitest"
import type { RestTaskResponse } from "~/api/generated/tasks"
import type { TaskFormDirtyFields } from "~/features/task/converters/taskFormValues2updateRequest"
import type { TaskFormValues } from "~/features/task/components/TaskForm/schema"
import { LanguageProviderContext } from "~/features/i18n/languageContext"
import { TaskAPIDataContext, type TaskAPIData } from "~/features/task/providers/TaskAPIDataProvider/context"
import { TaskActionContext, type TaskFormSaveResult } from "./context"
import { TaskActionProvider } from "."
import { emptyTaskFormValues } from "~/features/task/components/TaskForm/schema"

afterEach(() => {
  cleanup()
  vi.unstubAllGlobals()
})

const task: RestTaskResponse = {
  action_item_completed_count: 0,
  action_item_count: 0,
  actual_minutes: null,
  assignee_id: "user-1",
  can_add_action_item: true,
  can_add_to_today: true,
  can_update: true,
  created_at: "2026-10-10T00:00:00Z",
  description: "",
  due_date: null,
  estimate_editable: true,
  estimate_source: "manual",
  estimated_minutes: 30,
  id: "task-1",
  manual_estimated_minutes: 30,
  priority: "medium",
  progress: 0,
  project_id: null,
  project_name: null,
  remaining_days: null,
  revision: 7,
  started: false,
  status: "open",
  title: "Direct work",
  updated_at: "2026-10-10T00:00:00Z",
  user_id: "user-1",
}

function apiData(): TaskAPIData {
  const query = { data: undefined, error: null, isError: false, isLoading: false, refetch: vi.fn() }
  return {
    state: "ready",
    list: false,
    taskId: task.id,
    task,
    tasksQuery: query,
    taskQuery: query,
    actionItemsQuery: { ...query, data: [] },
    projectsQuery: { ...query, data: [] },
    optionsQuery: { ...query, data: { priorities: [] } },
  } as unknown as TaskAPIData
}

function Submitter({ onResult }: { onResult: (result: TaskFormSaveResult) => void }) {
  const actions = useContext(TaskActionContext)!
  return <>
    <button onClick={() => void actions.submit({ ...emptyTaskFormValues, manualEstimate: "1:30" }, { manualEstimate: true }).then(onResult)}>save</button>
    <button onClick={() => void actions.submit({ ...emptyTaskFormValues, manualEstimate: "1:30" }, { manualEstimate: true }, true).then(onResult)}>confirm</button>
  </>
}

function PendingDeleteSubmitter({ onResult }: { onResult: (result: TaskFormSaveResult) => void }) {
  const actions = useContext(TaskActionContext)!
  const firstSaveValues: TaskFormValues = {
    ...emptyTaskFormValues,
    title: task.title,
    manualEstimate: "0:45",
    actionItems: [
      { id: "child-1", title: "First child", estimatedMinutes: "0:30", priority: "medium", actualMinutes: "", pendingDelete: true },
      { id: "child-2", title: "Last child", estimatedMinutes: "0:30", priority: "medium", actualMinutes: "", pendingDelete: true },
    ],
  }
  const retryValues: TaskFormValues = { ...firstSaveValues, actionItems: [firstSaveValues.actionItems[1]!] }
  const dirty: TaskFormDirtyFields = { actionItems: [{ pendingDelete: true }, { pendingDelete: true }], manualEstimate: true }
  return <>
    <button onClick={() => void actions.submit(firstSaveValues, dirty).then(onResult)}>save deletes</button>
    <button onClick={() => void actions.submit(retryValues, { actionItems: [{ pendingDelete: true }], manualEstimate: true }).then(onResult)}>retry pending delete</button>
  </>
}

test("retries a Task estimate save with confirmation flag and same revision", async () => {
  const fetchMock = vi.fn()
    .mockResolvedValueOnce(new Response(JSON.stringify({ error: { details: [{ code: "todo_regeneration_required" }] } }), { status: 409 }))
    .mockResolvedValueOnce(new Response(JSON.stringify({ ...task, manual_estimated_minutes: 90, estimated_minutes: 90, revision: 8 }), { status: 200 }))
  vi.stubGlobal("fetch", fetchMock)
  const onResult = vi.fn()
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  render(<QueryClientProvider client={client}>
    <LanguageProviderContext.Provider value={{ language: "ja", setLanguage: vi.fn() }}>
      <TaskAPIDataContext.Provider value={apiData()}>
        <TaskActionProvider><Submitter onResult={onResult} /></TaskActionProvider>
      </TaskAPIDataContext.Provider>
    </LanguageProviderContext.Provider>
  </QueryClientProvider>)

  await act(async () => { screen.getByRole("button", { name: "save" }).click() })
  expect(onResult).toHaveBeenLastCalledWith(expect.objectContaining({ complete: false, regenerationConfirmationRequired: true }))
  await act(async () => { screen.getByRole("button", { name: "confirm" }).click() })

  expect(onResult).toHaveBeenLastCalledWith(expect.objectContaining({ complete: true, taskSaved: true }))
  const requests = fetchMock.mock.calls.map(([url, request]) => ({ url, request: request as RequestInit }))
  expect(requests.map(({ url }) => url)).toEqual(["/api/tasks/task-1", "/api/tasks/task-1"])
  expect(JSON.parse(String(requests[0]!.request.body))).toEqual({ manual_estimated_minutes: 90 })
  expect(JSON.parse(String(requests[1]!.request.body))).toEqual({ manual_estimated_minutes: 90, confirm_todo_regeneration: true })
  expect(new Headers(requests[0]!.request.headers).get("If-Match")).toBe('"7"')
  expect(new Headers(requests[1]!.request.headers).get("If-Match")).toBe('"7"')
})

test("does not adopt an external manual estimate during a pending final-delete refresh", async () => {
  let taskReads = 0
  const deleteCalls: string[] = []
  const fetchMock = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
    const url = new URL(String(input), "http://localhost")
    if (url.pathname === "/api/tasks/task-1" && (!init?.method || init.method === "GET")) {
      taskReads += 1
      if (taskReads === 1) return new Response(JSON.stringify({ task: { ...task, revision: 8 } }), { status: 200 })
      if (taskReads === 2) return new Response(JSON.stringify({ error: { code: "temporary_error" } }), { status: 500 })
      return new Response(JSON.stringify({ task: { ...task, revision: 9, manual_estimated_minutes: 90, estimated_minutes: 90 } }), { status: 200 })
    }
    if (url.pathname.startsWith("/api/tasks/task-1/action-items/") && init?.method === "DELETE") {
      deleteCalls.push(url.pathname)
      return deleteCalls.length === 1 ? new Response(null, { status: 204 }) : new Response(JSON.stringify({ error: { code: "temporary_error" } }), { status: 500 })
    }
    throw new Error(`Unexpected request: ${url.pathname} ${init?.method ?? "GET"}`)
  })
  vi.stubGlobal("fetch", fetchMock)
  const onResult = vi.fn()
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  render(<QueryClientProvider client={client}>
    <LanguageProviderContext.Provider value={{ language: "ja", setLanguage: vi.fn() }}>
      <TaskAPIDataContext.Provider value={apiData()}>
        <TaskActionProvider><PendingDeleteSubmitter onResult={onResult} /></TaskActionProvider>
      </TaskAPIDataContext.Provider>
    </LanguageProviderContext.Provider>
  </QueryClientProvider>)

  await act(async () => { screen.getByRole("button", { name: "save deletes" }).click() })
  expect(onResult).toHaveBeenLastCalledWith(expect.objectContaining({ complete: false, revisionRefreshError: true }))
  expect(deleteCalls).toHaveLength(2)
  await act(async () => { screen.getByRole("button", { name: "retry pending delete" }).click() })

  expect(onResult).toHaveBeenLastCalledWith(expect.objectContaining({ complete: false, conflict: true }))
  expect(deleteCalls).toHaveLength(2)
})
