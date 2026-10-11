import { useContext } from "react"
import { QueryClient, QueryClientProvider } from "@tanstack/react-query"
import { act, render, waitFor } from "@testing-library/react"
import { createMemoryRouter, RouterProvider } from "react-router"
import { describe, expect, it, vi } from "vitest"
import { ApiError } from "~/api/http"
import { getProjectsId, getProjectsIdMembers } from "~/api/generated/projects"
import { getProjectsIdSchedules, getScheduleBlueprintsId, getScheduleBlueprintsIdAssignees, getScheduleBlueprintsIdRevisions, getSchedulesBlueprintIdOccurrenceDate } from "~/api/generated/schedules"
import { LanguageProviderContext } from "~/features/i18n/languageContext"
import { ScheduleAPIDataContext, type ScheduleAPIData } from "~/features/schedule/providers/ScheduleAPIDataProvider/context"
import { ScheduleAPIDataProvider } from "~/features/schedule/providers/ScheduleAPIDataProvider"
import { scheduleRange } from "~/features/schedule/lib/scheduleRange"
import { useScheduleEditForm } from "~/features/schedule/hooks/useScheduleEditForm"
import { ScheduleActionProvider } from "~/features/schedule/providers/ScheduleActionProvider"
import { ScheduleNavigationProvider } from "~/features/schedule/providers/ScheduleNavigationProvider"
import ScheduleFormBoundary from "~/features/schedule/components/ScheduleFormBoundary"

vi.mock("~/api/generated/auth", () => ({
  getGetUsersMeQueryKey: () => ["users", "me"],
  getUsersMe: vi.fn(async () => { throw new Error("background user refresh failed") }),
  getTimezones: vi.fn(async () => { throw new Error("background timezone refresh failed") }),
}))
vi.mock("~/api/generated/projects", () => ({
  getProjects: vi.fn(async () => { throw new Error("background projects refresh failed") }),
  getProjectsId: vi.fn(),
  getProjectsIdMembers: vi.fn(),
  getGetProjectsIdMembersQueryKey: (id: string) => ["projects", id, "members"],
  getGetProjectsIdQueryKey: (id: string) => ["projects", id],
}))
vi.mock("~/api/generated/schedules", () => ({
  getGetScheduleBlueprintsIdAssigneesQueryKey: (id: string) => ["schedules", "assignees", id],
  getProjectsIdSchedules: vi.fn(),
  getScheduleBlueprintsId: vi.fn(),
  getScheduleBlueprintsIdAssignees: vi.fn(),
  getScheduleBlueprintsIdRevisions: vi.fn(),
  getSchedules: vi.fn(async () => { throw new Error("background schedules refresh failed") }),
  getSchedulesBlueprintIdOccurrenceDate: vi.fn(),
  getGetScheduleBlueprintsIdQueryKey: (id: string) => ["schedules", "blueprint", id],
}))
vi.mock("~/features/shared/notification", () => ({ notify: { success: vi.fn(), error: vi.fn() } }))

describe("ScheduleAPIDataProvider", () => {
  it("keeps cached calendar data ready when background refreshes fail", async () => {
    const anchor = "2026-10-11"
    const range = scheduleRange(anchor, "week", "UTC")
    const occurrence = {
      id: "event-1", blueprint_id: "blueprint-1", occurrence_date: "2026-10-10", title: "Cached event",
      start_at: "2026-10-10T09:00:00Z", end_at: "2026-10-10T10:00:00Z", timezone: "UTC",
      local_start: "2026-10-10T09:00:00", local_end: "2026-10-10T10:00:00", revision: 1,
    }
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false, staleTime: 0 } } })
    queryClient.setQueryData(["users", "me"], { user_id: "user-1", user_name: "User", timezone: "UTC" })
    queryClient.setQueryData(["schedules", "timezones"], { items: ["UTC"] })
    queryClient.setQueryData(["schedules", "projects"], [])
    queryClient.setQueryData(["schedules", "occurrences", "personal", range.from, range.to, false], [occurrence])
    let data: ScheduleAPIData | null | undefined
    function Capture() {
      data = useContext(ScheduleAPIDataContext)
      return null
    }
    const router = createMemoryRouter([{
      path: "/schedules",
      element: <ScheduleAPIDataProvider list><Capture /></ScheduleAPIDataProvider>,
    }], { initialEntries: [`/schedules?date=${anchor}&view=week`] })
    render(<QueryClientProvider client={queryClient}>
      <LanguageProviderContext.Provider value={{ language: "en", setLanguage: () => undefined }}>
        <RouterProvider router={router} />
      </LanguageProviderContext.Provider>
    </QueryClientProvider>)

    await waitFor(() => expect(data?.scheduleQuery.isError).toBe(true))
    expect(data?.state).toBe("ready")
    expect(data?.events.map((event) => event.title)).toEqual(["Cached event"])
    await act(async () => queryClient.clear())
  })

  it("keeps current-scope editing ready when Project members and optional blueprint data are forbidden", async () => {
    vi.mocked(getProjectsIdMembers).mockRejectedValue(new ApiError(403, {}))
    vi.mocked(getProjectsId).mockRejectedValue(new ApiError(403, {}))
    vi.mocked(getScheduleBlueprintsId).mockRejectedValue(new ApiError(403, {}))
    vi.mocked(getScheduleBlueprintsIdRevisions).mockRejectedValue(new ApiError(403, {}))
    vi.mocked(getScheduleBlueprintsIdAssignees).mockRejectedValue(new ApiError(403, {}))
    vi.mocked(getSchedulesBlueprintIdOccurrenceDate).mockResolvedValue({
      id: "override-1", blueprint_id: "blueprint-1", occurrence_date: "2026-10-10", project_id: "project-1",
      title: "Saved override", start_at: "2026-10-10T09:00:00Z", end_at: "2026-10-10T10:00:00Z",
      timezone: "UTC", revision: 2, can_update: true, can_assign: true,
    })
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false, staleTime: 0 } } })
    queryClient.setQueryData(["users", "me"], { user_id: "editor-1", user_name: "Editor", timezone: "UTC" })
    queryClient.setQueryData(["schedules", "timezones"], { items: ["UTC"] })
    queryClient.setQueryData(["schedules", "projects"], [])
    let data: ScheduleAPIData | null | undefined
    function Capture() {
      data = useContext(ScheduleAPIDataContext)
      return null
    }
    const router = createMemoryRouter([{
      path: "/schedules/:blueprintId/:occurrenceDate/edit",
      element: <ScheduleAPIDataProvider blueprintId="blueprint-1" occurrenceDate="2026-10-10"><Capture /></ScheduleAPIDataProvider>,
    }], { initialEntries: ["/schedules/blueprint-1/2026-10-10/edit"] })
    render(<QueryClientProvider client={queryClient}>
      <LanguageProviderContext.Provider value={{ language: "en", setLanguage: () => undefined }}>
        <RouterProvider router={router} />
      </LanguageProviderContext.Provider>
    </QueryClientProvider>)

    await waitFor(() => expect(data?.state).toBe("ready"))
    expect(data?.state).toBe("ready")
    expect(data?.event?.title).toBe("Saved override")
    expect(data?.blueprintAccessDenied).toBe(true)
    await act(async () => queryClient.clear())
  })

  it("waits for the initial blueprint and history before mounting the edit form", async () => {
    let resolveBlueprint!: (value: { revision: number; project_id: string }) => void
    let resolveRevisions!: (value: { items: Array<Record<string, unknown>> }) => void
    vi.mocked(getScheduleBlueprintsId).mockReturnValue(new Promise((resolve) => { resolveBlueprint = resolve }) as ReturnType<typeof getScheduleBlueprintsId>)
    vi.mocked(getScheduleBlueprintsIdRevisions).mockReturnValue(new Promise((resolve) => { resolveRevisions = resolve }) as ReturnType<typeof getScheduleBlueprintsIdRevisions>)
    vi.mocked(getScheduleBlueprintsIdAssignees).mockResolvedValue({ items: [] })
    vi.mocked(getSchedulesBlueprintIdOccurrenceDate).mockResolvedValue({
      id: "occurrence-1", blueprint_id: "blueprint-1", occurrence_date: "2026-10-10", title: "Source event",
      start_at: "2026-10-10T09:00:00Z", end_at: "2026-10-10T10:00:00Z", timezone: "UTC", revision: 2,
      source_revision_id: "revision-4", can_update: true, can_update_following: true,
    })
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false, staleTime: 0 } } })
    queryClient.setQueryData(["users", "me"], { user_id: "user-1", user_name: "User", timezone: "UTC" })
    queryClient.setQueryData(["schedules", "timezones"], { items: ["UTC"] })
    queryClient.setQueryData(["schedules", "projects"], [])
    let data: ScheduleAPIData | null | undefined
    let controller: ReturnType<typeof useScheduleEditForm> | undefined
    function Capture() {
      data = useContext(ScheduleAPIDataContext)
      return null
    }
    function EditForm() {
      controller = useScheduleEditForm()
      return null
    }
    const router = createMemoryRouter([{
      path: "/schedules/:blueprintId/:occurrenceDate/edit",
      element: <ScheduleAPIDataProvider blueprintId="blueprint-1" occurrenceDate="2026-10-10">
        <ScheduleNavigationProvider>
          <ScheduleActionProvider>
            <Capture />
            <ScheduleFormBoundary heading="schedules.form.editTitle" loadError="schedules.form.loadError"><EditForm /></ScheduleFormBoundary>
          </ScheduleActionProvider>
        </ScheduleNavigationProvider>
      </ScheduleAPIDataProvider>,
    }], { initialEntries: ["/schedules/blueprint-1/2026-10-10/edit"] })
    render(<QueryClientProvider client={queryClient}>
      <LanguageProviderContext.Provider value={{ language: "en", setLanguage: () => undefined }}>
        <RouterProvider router={router} />
      </LanguageProviderContext.Provider>
    </QueryClientProvider>)

    await waitFor(() => expect(data?.occurrenceQuery.isSuccess).toBe(true))
    expect(data?.state).toBe("loading")
    expect(controller).toBeUndefined()

    await act(async () => {
      resolveBlueprint({ revision: 4, project_id: "" })
      resolveRevisions({ items: [{
        revision_id: "revision-4", revision: 4, effective_from: "2026-01-01", title: "Source event",
        description: "", location: "", project_id: "", assignee_id: "", timezone: "Asia/Tokyo", all_day: false,
        local_start: "2026-10-10T09:00:00", local_end: "2026-10-10T10:00:00", frequency: "weekly", interval: 2,
        weekdays: ["SA"], end_date: "",
      }] })
    })

    await waitFor(() => expect(controller).toBeDefined())
    expect(data?.state).toBe("ready")
    expect(controller?.form.getValues("frequency")).toBe("weekly")
    expect(controller?.form.getValues("interval")).toBe(2)
    expect(controller?.form.getValues("weekdays")).toEqual(["SA"])
    expect(controller?.viewProps.showScopeSelector).toBe(true)
    await act(async () => queryClient.clear())
  })

  it("fetches every page of Project schedules with the generated pagination parameters", async () => {
    const first = {
      id: "event-1", blueprint_id: "blueprint-1", occurrence_date: "2026-10-10", title: "First page",
      start_at: "2026-10-10T09:00:00Z", end_at: "2026-10-10T10:00:00Z", timezone: "UTC",
    }
    const second = { ...first, id: "event-2", title: "Second page" }
    vi.mocked(getProjectsIdSchedules)
      .mockResolvedValueOnce({ items: [first], next_page_token: "page-2" })
      .mockResolvedValueOnce({ items: [second] })
    const anchor = "2026-10-11"
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
    queryClient.setQueryData(["users", "me"], { user_id: "user-1", user_name: "User", timezone: "UTC" })
    queryClient.setQueryData(["schedules", "timezones"], { items: ["UTC"] })
    queryClient.setQueryData(["schedules", "projects"], [])
    let data: ScheduleAPIData | null | undefined
    function Capture() {
      data = useContext(ScheduleAPIDataContext)
      return null
    }
    const router = createMemoryRouter([{
      path: "/schedules",
      element: <ScheduleAPIDataProvider list><Capture /></ScheduleAPIDataProvider>,
    }], { initialEntries: [`/schedules?date=${anchor}&view=week&project_id=project-1&show_skipped=true`] })
    render(<QueryClientProvider client={queryClient}>
      <LanguageProviderContext.Provider value={{ language: "en", setLanguage: () => undefined }}>
        <RouterProvider router={router} />
      </LanguageProviderContext.Provider>
    </QueryClientProvider>)

    await waitFor(() => expect(data?.events).toHaveLength(2))
    expect(data?.events.map((event) => event.title)).toEqual(["First page", "Second page"])
    expect(getProjectsIdSchedules).toHaveBeenCalledTimes(2)
    expect(getProjectsIdSchedules).toHaveBeenNthCalledWith(1, "project-1", expect.objectContaining({ include_skipped: true, page_size: 100, page_token: undefined }), expect.anything())
    expect(getProjectsIdSchedules).toHaveBeenNthCalledWith(2, "project-1", expect.objectContaining({ include_skipped: true, page_size: 100, page_token: "page-2" }), expect.anything())
    await act(async () => queryClient.clear())
  })
})
