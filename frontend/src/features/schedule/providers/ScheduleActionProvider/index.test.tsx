import { useContext, type ReactNode } from "react"
import { QueryClient, QueryClientProvider } from "@tanstack/react-query"
import { act, renderHook } from "@testing-library/react"
import { describe, expect, it, vi } from "vitest"
import { ApiError } from "~/api/http"
import { LanguageProviderContext } from "~/features/i18n/languageContext"
import { ScheduleAPIDataContext, type ScheduleAPIData, type ScheduleEvent } from "~/features/schedule/providers/ScheduleAPIDataProvider/context"
import { ScheduleActionContext } from "~/features/schedule/providers/ScheduleActionProvider/context"
import { ScheduleActionProvider } from "~/features/schedule/providers/ScheduleActionProvider"

vi.mock("~/features/shared/notification", () => ({ notify: { success: vi.fn(), error: vi.fn() } }))

const event: ScheduleEvent = {
  id: "virtual-event",
  ruleId: "rule-1",
  occurrenceDate: "2026-10-10",
  title: "Event",
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
  assigneeId: "user-1",
  revision: 0,
  state: "scheduled",
  persisted: false,
  permissions: {
    canUpdate: true, canDelete: true, canComplete: true, canSkip: true, canRestore: false,
    canAssign: true, canUpdateFollowing: true, canDeleteFollowing: true,
  },
}

describe("ScheduleActionProvider", () => {
  it("sends the occurrence revision, including zero for a virtual schedule", async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response(null, { status: 204 }))
    vi.stubGlobal("fetch", fetchMock)
    const { result } = renderActions()

    await act(async () => result.current!.complete(event))

    const request = fetchMock.mock.calls[0][1] as RequestInit
    expect(new Headers(request.headers).get("If-Match")).toBe('"0"')
  })

  it("propagates a precondition failure so the edit form can retain its draft and open conflict recovery", async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response(JSON.stringify({ error: { code: "revision_conflict" } }), { status: 412 }))
    vi.stubGlobal("fetch", fetchMock)
    const { result } = renderActions()

    await expect(act(async () => result.current!.complete(event))).rejects.toBeInstanceOf(ApiError)
    const request = fetchMock.mock.calls[0][1] as RequestInit
    expect(new Headers(request.headers).get("If-Match")).toBe('"0"')
  })
})

function renderActions() {
  const data = { projectOptions: [] } as unknown as ScheduleAPIData
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  function Wrapper({ children }: { children: ReactNode }) {
    return <QueryClientProvider client={queryClient}>
      <LanguageProviderContext.Provider value={{ language: "en", setLanguage: () => undefined }}>
        <ScheduleAPIDataContext.Provider value={data}>
          <ScheduleActionProvider>{children}</ScheduleActionProvider>
        </ScheduleAPIDataContext.Provider>
      </LanguageProviderContext.Provider>
    </QueryClientProvider>
  }
  return renderHook(() => useContext(ScheduleActionContext), { wrapper: Wrapper })
}
