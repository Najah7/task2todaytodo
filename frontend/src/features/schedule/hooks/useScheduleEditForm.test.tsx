import { type ReactNode } from "react"
import { QueryClient, QueryClientProvider } from "@tanstack/react-query"
import { act, render } from "@testing-library/react"
import { createMemoryRouter, RouterProvider } from "react-router"
import { describe, expect, it, vi } from "vitest"
import { ApiError } from "~/api/http"
import { LanguageProviderContext } from "~/features/i18n/languageContext"
import { ScheduleAPIDataContext, type ScheduleAPIData, type ScheduleEvent } from "~/features/schedule/providers/ScheduleAPIDataProvider/context"
import { ScheduleActionContext, type ScheduleActions } from "~/features/schedule/providers/ScheduleActionProvider/context"
import { ScheduleNavigationContext, type ScheduleNavigation } from "~/features/schedule/providers/ScheduleNavigationProvider/context"
import { useScheduleEditForm } from "~/features/schedule/hooks/useScheduleEditForm"

const event: ScheduleEvent = {
  id: "schedule-1",
  ruleId: "rule-1",
  occurrenceDate: "2026-10-10",
  title: "Original title",
  description: "",
  location: "",
  start: new Date("2026-10-10T09:00:00Z"),
  end: new Date("2026-10-10T10:00:00Z"),
  localStart: "2026-10-10T09:00:00",
  localEnd: "2026-10-10T10:00:00",
  startDate: "2026-10-10",
  endDate: "2026-10-10",
  timezone: "UTC",
  allDay: false,
  projectId: "",
  projectTitle: "",
  assigneeId: "",
  revision: 9,
  state: "scheduled",
  persisted: true,
  recurrence: { frequency: "once", interval: 1, weekdays: [], endDate: "" },
  permissions: {
    canUpdate: false,
    canDelete: true,
    canComplete: true,
    canSkip: true,
    canRestore: false,
    canAssign: false,
    canUpdateFollowing: true,
    canDeleteFollowing: true,
  },
}

describe("useScheduleEditForm", () => {
  it("uses following permissions and retains the draft and revision baseline after a 412", async () => {
    const update = vi.fn().mockRejectedValue(new ApiError(412, { error: "conflict" }))
    let controller: ReturnType<typeof useScheduleEditForm> | undefined
    function Harness() {
      controller = useScheduleEditForm()
      return null
    }
    const revision = {
      revisionId: "source-revision-1", revision: 4, effectiveFrom: "2026-10-01", superseded: false, deleted: false,
      title: "Original title", description: "", location: "", projectId: "", assigneeId: "", timezone: "UTC", allDay: false,
      localStart: "2026-10-10T09:00:00", localEnd: "2026-10-10T10:00:00", frequency: "weekly" as const,
      interval: 1, weekdays: ["SA"], endDate: "",
    }
    const data = { event: { ...event, sourceRevisionId: "source-revision-1" }, timezone: "UTC", timezones: ["UTC"], projectOptions: [], ruleRevision: revision, ruleRevisionNumber: 4, ruleAccessDenied: false } as unknown as ScheduleAPIData
    const reloadLatest = vi.fn().mockImplementation(async (_event, _scope, values) => ({
      values,
      occurrenceRevision: 10,
      ruleRevision: undefined,
      followingAvailable: false,
    }))
    const actions = { busy: false, update, reloadLatest } as unknown as ScheduleActions
    const navigation = { afterSave: vi.fn(), cancel: vi.fn() } as unknown as ScheduleNavigation
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
    const router = createMemoryRouter([{ path: "/schedules/:id/edit", element: <Harness /> }], { initialEntries: ["/schedules/rule-1/edit"] })
    function Wrapper({ children }: { children: ReactNode }) {
      return <QueryClientProvider client={queryClient}>
        <LanguageProviderContext.Provider value={{ language: "en", setLanguage: () => undefined }}>
          <ScheduleAPIDataContext.Provider value={data}>
            <ScheduleActionContext.Provider value={actions}>
              <ScheduleNavigationContext.Provider value={navigation}>{children}</ScheduleNavigationContext.Provider>
            </ScheduleActionContext.Provider>
          </ScheduleAPIDataContext.Provider>
        </LanguageProviderContext.Provider>
      </QueryClientProvider>
    }
    render(<Wrapper><RouterProvider router={router} /></Wrapper>)

    expect(controller?.viewProps.canEditContent).toBe(false)
    expect(controller?.viewProps.showScopeSelector).toBe(true)
    act(() => {
      controller!.form.setValue("scope", "following", { shouldDirty: true })
      controller!.form.setValue("frequency", "weekly", { shouldDirty: true })
      controller!.form.setValue("weekdays", ["MO"], { shouldDirty: true })
      controller!.form.setValue("title", "Draft title", { shouldDirty: true })
    })
    expect(controller?.viewProps.canEditContent).toBe(true)
    expect(controller?.viewProps.showRecurrenceFields).toBe(true)

    await act(async () => controller!.viewProps.onSubmit())

    expect(update).toHaveBeenCalledWith(expect.objectContaining({ revision: 9, ruleRevision: 4, scope: "following" }))
    expect(controller?.form.getValues("title")).toBe("Draft title")
    expect(controller?.viewProps.hasConflict).toBe(true)

    act(() => {
      controller!.form.setValue("scope", "current", { shouldDirty: true })
      controller!.viewProps.onRequestReload()
    })
    await act(async () => controller!.viewProps.reloadConfirmation.onConfirm())
    expect(reloadLatest).toHaveBeenCalledWith(expect.anything(), "current", expect.objectContaining({
      title: "Draft title",
      frequency: "weekly",
      weekdays: ["MO"],
    }))
    expect(controller?.form.getValues("frequency")).toBe("weekly")
    expect(controller?.viewProps.showScopeSelector).toBe(false)
  })
})
