import { useContext, useMemo, useState } from "react"
import { useSearchParams } from "react-router"
import { Temporal } from "@js-temporal/polyfill"
import { useQuery } from "@tanstack/react-query"
import { getGetUsersMeQueryOptions } from "~/api/generated/auth"
import type { AllocationEditorValues } from "~/features/allocation/components/AllocationEditor/schema"
import type { DailyActiveTimeValues } from "~/features/allocation/components/DailyActiveTimeEditor/schema"
import AllocationEditor from "~/features/allocation/components/AllocationEditor"
import DailyActiveTimeEditor from "~/features/allocation/components/DailyActiveTimeEditor"
import WeeklyPlanBoard from "~/features/allocation/components/WeeklyPlanBoard"
import { allocationWeeks2weeklyPlanBoardProps } from "~/features/allocation/converters/allocationWeeks2weeklyPlanBoardProps"
import { dateAndMinute2localDateTime } from "~/features/allocation/converters/localDateTimes"
import { localDateTimeToInstant } from "~/features/shared/calendar/dateTime"
import { dateInTimezone, isDateKey, mondayOf } from "~/features/allocation/lib/week"
import { AllocationAPIDataProvider } from "~/features/allocation/providers/AllocationAPIDataProvider"
import { AllocationActionProvider } from "~/features/allocation/providers/AllocationActionProvider"
import { AllocationNavigationProvider } from "~/features/allocation/providers/AllocationNavigationProvider"
import { AllocationAPIDataContext } from "~/features/allocation/providers/AllocationAPIDataProvider/context"
import { AllocationActionContext } from "~/features/allocation/providers/AllocationActionProvider/context"
import { AllocationNavigationContext } from "~/features/allocation/providers/AllocationNavigationProvider/context"
import PageHeading from "~/features/shared/components/PageHeading"
import { useI18n } from "~/features/i18n/hooks"

type EditorState = { kind: "create"; projectId: string; date: string; startMinute: number; endMinute: number } | { kind: "edit"; allocation: ReturnType<typeof allocationWeeks2weeklyPlanBoardProps>[number]["allocations"][number] }

function defaultWeekStart(timezone: string) {
  return mondayOf(dateInTimezone(new Date(), timezone))
}

export default function WeeklyPlanPage() {
  const [searchParams] = useSearchParams()
  const userQuery = useQuery({ ...getGetUsersMeQueryOptions(), retry: false })
  const candidate = searchParams.get("week_start")
  const weekStart = candidate && isDateKey(candidate) ? mondayOf(candidate) : defaultWeekStart(userQuery.data?.timezone || Intl.DateTimeFormat().resolvedOptions().timeZone)
  return <AllocationAPIDataProvider weekStart={weekStart}>
    <AllocationNavigationProvider weekStart={weekStart}>
      <AllocationActionProvider><WeeklyPlanContents /></AllocationActionProvider>
    </AllocationNavigationProvider>
  </AllocationAPIDataProvider>
}

function WeeklyPlanContents() {
  const i18n = useI18n()
  const dataContext = useContext(AllocationAPIDataContext)
  const actionsContext = useContext(AllocationActionContext)
  const navigationContext = useContext(AllocationNavigationContext)
  if (!dataContext || !actionsContext || !navigationContext) throw new Error("Weekly Plan providers are required")
  const data = dataContext
  const actions = actionsContext
  const navigation = navigationContext
  const [editor, setEditor] = useState<EditorState>()
  const [activeTimeDate, setActiveTimeDate] = useState<string>()
  const board = useMemo(() => data.week ? allocationWeeks2weeklyPlanBoardProps([data.week])[0] : undefined, [data.week])
  const model = editor?.kind === "edit" ? editor.allocation : undefined
  const initialValues: AllocationEditorValues = useMemo(() => editor?.kind === "edit"
    ? { projectId: editor.allocation.projectId, startAt: editor.allocation.startAt, endAt: editor.allocation.endAt, repeatWeekly: false }
    : { projectId: editor?.kind === "create" ? editor.projectId : "", startAt: editor ? dateAndMinute2localDateTime(editor.date, editor.startMinute) : "", endAt: editor ? dateAndMinute2localDateTime(editor.date, editor.endMinute) : "", repeatWeekly: false }, [editor])

  if (data.state !== "ready" || !board) return <>
    <PageHeading messageKey="page.weeklyPlan.title" descriptionKey="weeklyPlan.description" />
    <p role={data.state === "error" ? "alert" : undefined} className="text-body">{i18n(data.state === "loading" ? "weeklyPlan.loading" : data.state === "forbidden" ? "weeklyPlan.forbidden" : "weeklyPlan.loadError")}</p>
    {data.state !== "loading" && <button type="button" onClick={() => { void data.refetch() }}>{i18n("weeklyPlan.retry")}</button>}
  </>

  const activeTimeDay = activeTimeDate ? board.days.find((day) => day.date === activeTimeDate) : undefined

  async function saveAllocation(values: AllocationEditorValues) {
    const startAt = localDateTimeToInstant(values.startAt, data.timezone)
    const endAt = localDateTimeToInstant(values.endAt, data.timezone)
    if (model) {
      await actions.update(model.ruleId, model.occurrenceDate, model.revision, {
        project_id: values.projectId,
        start_at: startAt,
        end_at: endAt,
        scope: values.repeatWeekly ? "following" : "current",
        ...(values.repeatWeekly ? { rule_revision: model.ruleRevision } : {}),
      })
    } else {
      await actions.create({ project_id: values.projectId, timezone: data.timezone, start_at: startAt, end_at: endAt, repeat_weekly: values.repeatWeekly })
    }
    setEditor(undefined)
  }

  async function deleteAllocation(scope: "current" | "following") {
    if (!model) return
    await actions.remove(model.ruleId, model.occurrenceDate, model.revision, model.ruleRevision, scope)
    setEditor(undefined)
  }

  async function saveActiveTime(values: DailyActiveTimeValues) {
    if (!activeTimeDate || !activeTimeDay) return
    const date = Temporal.PlainDate.from(activeTimeDate)
    const endDate = values.nextDay ? date.add({ days: 1 }) : date
    await actions.setActiveTime(activeTimeDate, activeTimeDay.activeRevision, {
      active_start_local: activeTimeDate + "T" + values.start,
      active_end_local: endDate.toString() + "T" + values.end,
      timezone: data.timezone,
    })
    setActiveTimeDate(undefined)
  }

  async function resetActiveTime() {
    if (!activeTimeDate || !activeTimeDay) return
    await actions.resetActiveTime(activeTimeDate, activeTimeDay.activeRevision)
    setActiveTimeDate(undefined)
  }

  return <>
    <PageHeading messageKey="page.weeklyPlan.title" descriptionKey="weeklyPlan.description" />
    <WeeklyPlanBoard {...board} canEdit={data.isPaid} canEditActiveTime showAllocationData={data.isPaid} busy={actions.busy}
      onChangeWeek={navigation.changeWeek} onEditPlan={navigation.editPlan}
      onCreateAllocation={(projectId, date, startMinute, endMinute) => setEditor({ kind: "create", projectId, date, startMinute, endMinute })}
      onEditAllocation={(allocation) => setEditor({ kind: "edit", allocation })}
      onEditActiveTime={setActiveTimeDate} />
    <AllocationEditor open={Boolean(editor)} timezone={data.timezone} projects={model && !data.projects.some((project) => project.id === model.projectId) ? [...data.projects, { id: model.projectId, title: model.projectTitle }] : data.projects} initialValues={initialValues} allocation={model} busy={actions.busy} onSave={saveAllocation} onDelete={deleteAllocation} onCancel={() => setEditor(undefined)} />
    {activeTimeDate && activeTimeDay && <DailyActiveTimeEditor open date={activeTimeDate} start={activeTimeDay.activeStartAt || data.user?.active_start_time || "07:00"} end={activeTimeDay.activeEndAt || data.user?.active_end_time || "23:00"} timezone={data.timezone} revision={activeTimeDay.activeRevision} hasException={activeTimeDay.hasActiveException} busy={actions.busy} onSave={saveActiveTime} onReset={resetActiveTime} onCancel={() => setActiveTimeDate(undefined)} />}
  </>
}
