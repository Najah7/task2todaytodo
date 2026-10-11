import { useContext, useState, type ReactNode } from "react"
import { Temporal } from "@js-temporal/polyfill"
import { useLocation, useNavigate, useSearchParams } from "react-router"
import { scheduleSearchParams2state, scheduleState2searchParams } from "~/features/schedule/converters/scheduleSearchParams"
import { ScheduleAPIDataContext } from "~/features/schedule/providers/ScheduleAPIDataProvider/context"
import { ScheduleNavigationContext, type ScheduleSlotPrefill, type ScheduleView } from "./context"

type Props = { children: ReactNode }

export function ScheduleNavigationProvider({ children }: Props) {
  const data = useContext(ScheduleAPIDataContext)!
  const [searchParams, setSearchParams] = useSearchParams()
  const navigate = useNavigate()
  const location = useLocation()
  const [pendingPrefill, setPendingPrefill] = useState<ScheduleSlotPrefill>()
  const today = Temporal.Now.plainDateISO(data.timezone || "UTC").toString()
  const state = scheduleSearchParams2state(searchParams, today)
  const routePrefill = (location.state as { schedulePrefill?: ScheduleSlotPrefill } | null)?.schedulePrefill

  function change(changes: Partial<typeof state>, replace = false) {
    const next = { ...state, ...changes }
    setSearchParams(scheduleState2searchParams(next, searchParams), { replace })
  }

  function navigateToList() {
    navigate(`/schedules?${scheduleState2searchParams(state, searchParams).toString()}`, { replace: true })
  }

  const navigation = {
    state,
    slotPrefill: routePrefill ?? pendingPrefill,
    setDate: (date: string, options?: { replace?: boolean }) => change({ date }, options?.replace),
    setView: (view: ScheduleView, date?: string) => change({ view, ...(date ? { date } : {}) }),
    openDay: (date: string) => change({ date, view: "day" }),
    setProject: (projectId?: string) => change({ projectId }),
    setShowSkipped: (showSkipped: boolean) => change({ showSkipped }),
    previous: () => change({ date: shiftDate(state.date, state.view, -1) }),
    next: () => change({ date: shiftDate(state.date, state.view, 1) }),
    today: () => change({ date: Temporal.Now.plainDateISO(data.timezone || "UTC").toString() }),
    create: (prefill?: ScheduleSlotPrefill) => {
      setPendingPrefill(undefined)
      navigate(`/schedules/new?${scheduleState2searchParams(state, searchParams).toString()}`, {
        state: prefill ? { schedulePrefill: prefill } : undefined,
      })
    },
    edit: (blueprintId: string, occurrenceDate: string) => {
      navigate(`/schedules/${encodeURIComponent(blueprintId)}/${encodeURIComponent(occurrenceDate)}/edit?${scheduleState2searchParams(state, searchParams).toString()}`)
    },
    list: navigateToList,
    cancel: navigateToList,
    afterSave: navigateToList,
    setSlotPrefill: setPendingPrefill,
  }

  return <ScheduleNavigationContext.Provider value={navigation}>{children}</ScheduleNavigationContext.Provider>
}

function shiftDate(value: string, view: ScheduleView, direction: number): string {
  const date = Temporal.PlainDate.from(value)
  if (view === "day") return date.add({ days: direction }).toString()
  if (view === "week") return date.add({ days: 7 * direction }).toString()
  return date.add({ months: direction }).toString()
}
