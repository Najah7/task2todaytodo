import { useContext, type ReactNode } from "react"
import { QueryClient, QueryClientProvider } from "@tanstack/react-query"
import { act, renderHook } from "@testing-library/react"
import { afterEach, describe, expect, it, vi } from "vitest"
import { ApiError } from "~/api/http"
import { getListInboxQueryKey, type RestInboxItemResponse } from "~/api/generated/inbox"
import { LanguageProviderContext } from "~/features/i18n/languageContext"
import { emptyInboxQuickCreateValues } from "~/features/inbox/components/InboxQuickCreate/schema"
import { InboxAPIDataContext, type InboxAPIData } from "~/features/inbox/providers/InboxAPIDataProvider/context"
import { InboxActionContext } from "~/features/inbox/providers/InboxActionProvider/context"
import { InboxActionProvider } from "~/features/inbox/providers/InboxActionProvider"

vi.mock("~/features/shared/notification", () => ({ notify: { success: vi.fn(), error: vi.fn() } }))

const taskItem: RestInboxItemResponse = {
  type: "task",
  id: "task-1",
  title: "Write tests",
  project_id: null,
  project_name: null,
  created_at: "2026-10-12T00:00:00Z",
  schedule: null,
  task: {
    id: "task-1",
    description: "",
    assignee_id: "user-1",
    due_date: null,
    remaining_days: null,
    estimated_minutes: null,
    estimate_source: "none",
    actual_minutes: null,
    progress: 0,
    priority: "normal",
    status: "todo",
    action_item_count: 0,
    action_item_completed_count: 0,
    revision: 7,
    can_update: true,
    can_assign: true,
  },
}

const virtualScheduleItem: RestInboxItemResponse = {
  type: "schedule",
  id: "blueprint-2/2026-10-13",
  title: "Focus time",
  project_id: null,
  project_name: null,
  created_at: "2026-10-01T00:00:00Z",
  task: null,
  schedule: {
    occurrence_id: null,
    blueprint_id: "blueprint-2",
    occurrence_date: "2026-10-13",
    source_revision_id: "revision-1",
    assignee_id: "user-1",
    description: "",
    location: "",
    timezone: "Asia/Tokyo",
    all_day: true,
    completed: false,
    skipped: false,
    can_update: true,
    can_assign: true,
    can_update_following: true,
    start_at: "2026-10-12T15:00:00Z",
    end_at: "2026-10-13T15:00:00Z",
    revision: 0,
  },
}

afterEach(() => {
  vi.unstubAllGlobals()
  vi.clearAllMocks()
})

describe("InboxActionProvider", () => {
  it("completes a task with its current revision", async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response(JSON.stringify({ message: "ok" }), { status: 200 }))
    vi.stubGlobal("fetch", fetchMock)
    const { result } = renderActions()

    await act(async () => result.current!.complete(taskItem))

    const [url, request] = fetchMock.mock.calls[0] as [string, RequestInit]
    expect(url).toBe("/api/tasks/task-1:complete")
    expect(request.method).toBe("POST")
    expect(new Headers(request.headers).get("If-Match")).toBe('"7"')
  })

  it("completes a virtual schedule occurrence with revision zero and its blueprint/date key", async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response(null, { status: 204 }))
    vi.stubGlobal("fetch", fetchMock)
    const { result } = renderActions()

    await act(async () => result.current!.complete(virtualScheduleItem))

    const [url, request] = fetchMock.mock.calls[0] as [string, RequestInit]
    expect(url).toBe("/api/schedules/blueprint-2/2026-10-13:complete")
    expect(request.method).toBe("POST")
    expect(new Headers(request.headers).get("If-Match")).toBe('"0"')
  })

  it("invalidates Inbox once, shows one error, and rejects after a revision conflict", async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response(JSON.stringify({ error: { code: "revision_conflict" } }), { status: 409 }))
    vi.stubGlobal("fetch", fetchMock)
    const { result, queryClient } = renderActions()
    const inboxKey = getListInboxQueryKey({ page_size: 50 })
    queryClient.setQueryData(inboxKey, { items: [], counts: { all: 0, task: 0, schedule: 0 } })

    await expect(act(async () => result.current!.complete(taskItem))).rejects.toBeInstanceOf(ApiError)

    expect(queryClient.getQueryState(inboxKey)?.isInvalidated).toBe(true)
    const { notify } = await import("~/features/shared/notification")
    expect(notify.error).toHaveBeenCalledTimes(1)
  })

  it("creates a one-off multi-day all-day schedule using the shared schedule request conversion", async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response(JSON.stringify({ id: "blueprint-3" }), { status: 201 }))
    vi.stubGlobal("fetch", fetchMock)
    const { result } = renderActions()

    await act(async () => result.current!.createSchedule({
      ...emptyInboxQuickCreateValues,
      title: "Conference",
      projectId: "project-1",
      startDate: "2026-10-12",
      endDate: "2026-10-14",
      allDay: true,
      timezone: "Asia/Tokyo",
    }))

    const [url, request] = fetchMock.mock.calls[0] as [string, RequestInit]
    expect(url).toBe("/api/schedule-blueprints")
    expect(JSON.parse(String(request.body))).toMatchObject({
      title: "Conference",
      project_id: "project-1",
      assignee_id: "user-1",
      frequency: "once",
      interval: 0,
      all_day: true,
      local_start: "2026-10-12T00:00:00",
      local_end: "2026-10-15T00:00:00",
      timezone: "Asia/Tokyo",
    })
  })
})

function renderActions() {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  const data = {
    state: "ready",
    inboxQuery: { refetch: vi.fn() },
    projectsQuery: { refetch: vi.fn() },
    userQuery: { refetch: vi.fn() },
    projects: [],
    timezone: "Asia/Tokyo",
    userId: "user-1",
  } as unknown as InboxAPIData
  function Wrapper({ children }: { children: ReactNode }) {
    return <QueryClientProvider client={queryClient}>
      <LanguageProviderContext.Provider value={{ language: "en", setLanguage: () => undefined }}>
        <InboxAPIDataContext.Provider value={data}>
          <InboxActionProvider>{children}</InboxActionProvider>
        </InboxAPIDataContext.Provider>
      </LanguageProviderContext.Provider>
    </QueryClientProvider>
  }
  return { ...renderHook(() => useContext(InboxActionContext), { wrapper: Wrapper }), queryClient }
}
