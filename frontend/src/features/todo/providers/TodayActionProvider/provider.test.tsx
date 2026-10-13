import { useContext, type ReactNode } from "react"
import { QueryClient, QueryClientProvider } from "@tanstack/react-query"
import { act, renderHook } from "@testing-library/react"
import { expect, it, vi } from "vitest"
import { getGetTodayTodoQueryKey, type RestTodoTodayResponse } from "~/api/generated/todo"
import { getGetTodoCompletionMetricsQueryKey } from "~/api/generated/metrics"
import { getListInboxQueryKey } from "~/api/generated/inbox"
import { getGetTasksQueryKey } from "~/api/generated/tasks"
import { getGetProjectsQueryKey } from "~/api/generated/projects"
import { notify } from "~/features/shared/notification"
import { LanguageProviderContext } from "~/features/i18n/languageContext"
import { TodayActionContext } from "~/features/todo/providers/TodayActionProvider/context"
import { TodayActionProvider } from "~/features/todo/providers/TodayActionProvider/provider"

vi.mock("~/features/shared/notification", () => ({ notify: { success: vi.fn(), error: vi.fn() } }))

it("invalidates live Schedule and Inbox query families after completing a Today Schedule", async () => {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  const todayKey = getGetTodayTodoQueryKey()
  const scheduleRowsKey = ["schedules", "occurrences", "personal", "2026-10-12", false]
  const scheduleDetailKey = ["schedules", "occurrence", "rule-1", "2026-10-12"]
  const inboxKey = getListInboxQueryKey()
  const snapshot: RestTodoTodayResponse = {
    date: "2026-10-12",
    revision: 1,
    items: [{ id: "todo-schedule", kind: "schedule", schedule_rule_id: "rule-1", occurrence_date: "2026-10-12", project_id: "project-1", can_complete: true }],
  }
  queryClient.setQueryData(todayKey, snapshot)
  queryClient.setQueryData(scheduleRowsKey, { items: [] })
  queryClient.setQueryData(scheduleDetailKey, { item: {} })
  queryClient.setQueryData(inboxKey, { items: [] })
  const completedSnapshot = { ...snapshot, revision: 2, items: snapshot.items?.map((item) => ({ ...item, completed: true })) }
  const fetchMock = vi.fn().mockResolvedValue(new Response(JSON.stringify(completedSnapshot), { status: 200, headers: { "content-type": "application/json" } }))
  vi.stubGlobal("fetch", fetchMock)

  function Wrapper({ children }: { children: ReactNode }) {
    return <QueryClientProvider client={queryClient}>
      <LanguageProviderContext.Provider value={{ language: "en", setLanguage: () => undefined }}>
        <TodayActionProvider>{children}</TodayActionProvider>
      </LanguageProviderContext.Provider>
    </QueryClientProvider>
  }

  const { result } = renderHook(() => useContext(TodayActionContext), { wrapper: Wrapper })
  await act(async () => { await result.current!.complete("todo-schedule") })

  expect(queryClient.getQueryState(scheduleRowsKey)?.isInvalidated).toBe(true)
  expect(queryClient.getQueryState(scheduleDetailKey)?.isInvalidated).toBe(true)
  expect(queryClient.getQueryState(inboxKey)?.isInvalidated).toBe(true)
  vi.unstubAllGlobals()
  queryClient.clear()
})

it("invalidates Task, Inbox, and Project data after completing a direct Task row", async () => {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  const todayKey = getGetTodayTodoQueryKey()
  const taskKey = getGetTasksQueryKey()
  const inboxKey = getListInboxQueryKey()
  const projectsKey = getGetProjectsQueryKey()
  const snapshot: RestTodoTodayResponse = {
    date: "2026-10-12",
    revision: 1,
    items: [{ id: "todo-task", kind: "task", task_id: "task-1", project_id: "project-1", can_complete: true }],
  }
  queryClient.setQueryData(todayKey, snapshot)
  queryClient.setQueryData(taskKey, { items: [] })
  queryClient.setQueryData(inboxKey, { items: [] })
  queryClient.setQueryData(projectsKey, [])
  const completedSnapshot = { ...snapshot, revision: 2, items: snapshot.items?.map((item) => ({ ...item, completed: true })) }
  const fetchMock = vi.fn().mockResolvedValue(new Response(JSON.stringify(completedSnapshot), { status: 200, headers: { "content-type": "application/json" } }))
  vi.stubGlobal("fetch", fetchMock)

  function Wrapper({ children }: { children: ReactNode }) {
    return <QueryClientProvider client={queryClient}>
      <LanguageProviderContext.Provider value={{ language: "en", setLanguage: () => undefined }}>
        <TodayActionProvider>{children}</TodayActionProvider>
      </LanguageProviderContext.Provider>
    </QueryClientProvider>
  }

  const { result } = renderHook(() => useContext(TodayActionContext), { wrapper: Wrapper })
  await act(async () => { await result.current!.complete("todo-task", 35) })

  expect(queryClient.getQueryState(taskKey)?.isInvalidated).toBe(true)
  expect(queryClient.getQueryState(inboxKey)?.isInvalidated).toBe(true)
  expect(queryClient.getQueryState(projectsKey)?.isInvalidated).toBe(true)
  vi.unstubAllGlobals()
  queryClient.clear()
})

it("regenerates with the current revision, replaces the Todo snapshot, and invalidates completion metrics", async () => {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  const todayKey = getGetTodayTodoQueryKey()
  const metrics7Key = getGetTodoCompletionMetricsQueryKey({ days: 7 })
  const metrics30Key = getGetTodoCompletionMetricsQueryKey({ days: 30 })
  const original: RestTodoTodayResponse = { date: "2026-10-12", revision: 7, items: [{ id: "manual-row", kind: "task", completed: true }] }
  const regenerated: RestTodoTodayResponse = { date: "2026-10-12", revision: 8, items: [{ id: "new-row", kind: "task", completed: false }] }
  queryClient.setQueryData(todayKey, original)
  queryClient.setQueryData(metrics7Key, { completed_items: 1 })
  queryClient.setQueryData(metrics30Key, { completed_items: 1 })
  const fetchMock = vi.fn().mockResolvedValue(new Response(JSON.stringify(regenerated), { status: 200, headers: { "content-type": "application/json" } }))
  vi.stubGlobal("fetch", fetchMock)

  function Wrapper({ children }: { children: ReactNode }) {
    return <QueryClientProvider client={queryClient}>
      <LanguageProviderContext.Provider value={{ language: "en", setLanguage: () => undefined }}>
        <TodayActionProvider>{children}</TodayActionProvider>
      </LanguageProviderContext.Provider>
    </QueryClientProvider>
  }

  const { result } = renderHook(() => useContext(TodayActionContext), { wrapper: Wrapper })
  let saved = false
  await act(async () => { saved = await result.current!.regenerate() })

  expect(saved).toBe(true)
  expect(fetchMock).toHaveBeenCalledOnce()
  expect(fetchMock.mock.calls[0]?.[0]).toBe("/api/today-todo:regenerate")
  const [, request] = fetchMock.mock.calls[0] as [string, RequestInit]
  expect(request.method).toBe("POST")
  expect(new Headers(request.headers).get("If-Match")).toBe('"7"')
  expect(queryClient.getQueryData(todayKey)).toEqual(regenerated)
  expect(queryClient.getQueryState(metrics7Key)?.isInvalidated).toBe(true)
  expect(queryClient.getQueryState(metrics30Key)?.isInvalidated).toBe(true)
  expect(notify.success).toHaveBeenCalledWith("Today's list was regenerated.")
  vi.unstubAllGlobals()
  queryClient.clear()
})

it("keeps the saved Todo snapshot and reports an error when regeneration fails", async () => {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  const todayKey = getGetTodayTodoQueryKey()
  const original: RestTodoTodayResponse = { date: "2026-10-12", revision: 9, items: [{ id: "manual-row", kind: "action_item" }] }
  queryClient.setQueryData(todayKey, original)
  const fetchMock = vi.fn().mockResolvedValue(new Response(JSON.stringify({ error: { message: "failed" } }), { status: 500, headers: { "content-type": "application/json" } }))
  vi.stubGlobal("fetch", fetchMock)

  function Wrapper({ children }: { children: ReactNode }) {
    return <QueryClientProvider client={queryClient}>
      <LanguageProviderContext.Provider value={{ language: "en", setLanguage: () => undefined }}>
        <TodayActionProvider>{children}</TodayActionProvider>
      </LanguageProviderContext.Provider>
    </QueryClientProvider>
  }

  const { result } = renderHook(() => useContext(TodayActionContext), { wrapper: Wrapper })
  let saved = true
  await act(async () => { saved = await result.current!.regenerate() })

  expect(saved).toBe(false)
  expect(queryClient.getQueryData(todayKey)).toEqual(original)
  expect(notify.error).toHaveBeenCalledWith("Today's list could not be updated. Reload and try again.")
  vi.unstubAllGlobals()
  queryClient.clear()
})
