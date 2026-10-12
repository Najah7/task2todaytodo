import { useContext, useMemo, useState } from "react"
import { useSearchParams } from "react-router"
import { Temporal } from "@js-temporal/polyfill"
import { useQuery } from "@tanstack/react-query"
import { getGetUsersMeQueryOptions } from "~/api/generated/auth"
import type { AllocationEditorValues } from "~/features/allocation/components/AllocationEditor/schema"
import type { DailySleepValues } from "~/features/allocation/components/DailySleepEditor/schema"
import AllocationEditor from "~/features/allocation/components/AllocationEditor"
import DailySleepEditor from "~/features/allocation/components/DailySleepEditor"
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
  const [sleepDate, setSleepDate] = useState<string>()
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

  const sleepDay = sleepDate ? board.days.find((day) => day.date === sleepDate) : undefined

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

  async function saveSleep(values: DailySleepValues) {
    if (!sleepDate || !sleepDay) return
    const date = Temporal.PlainDate.from(sleepDate)
    const bedtimeDate = values.nextDay ? date.add({ days: 1 }) : date
    await actions.setSleep(sleepDate, sleepDay.sleepRevision, {
      wake_local: sleepDate + "T" + values.wake,
      bedtime_local: bedtimeDate.toString() + "T" + values.bedtime,
      timezone: data.timezone,
    })
    setSleepDate(undefined)
  }

  async function resetSleep() {
    if (!sleepDate || !sleepDay) return
    await actions.resetSleep(sleepDate, sleepDay.sleepRevision)
    setSleepDate(undefined)
  }

  return <>
    <PageHeading messageKey="page.weeklyPlan.title" descriptionKey="weeklyPlan.description" />
    <WeeklyPlanBoard {...board} canEdit busy={actions.busy}
      onChangeWeek={navigation.changeWeek} onEditPlan={navigation.editPlan}
      onCreateAllocation={(projectId, date, startMinute, endMinute) => setEditor({ kind: "create", projectId, date, startMinute, endMinute })}
      onEditAllocation={(allocation) => setEditor({ kind: "edit", allocation })}
      onEditSleep={setSleepDate} />
    <AllocationEditor open={Boolean(editor)} timezone={data.timezone} projects={model && !data.projects.some((project) => project.id === model.projectId) ? [...data.projects, { id: model.projectId, title: model.projectTitle }] : data.projects} initialValues={initialValues} allocation={model} busy={actions.busy} onSave={saveAllocation} onDelete={deleteAllocation} onCancel={() => setEditor(undefined)} />
    {sleepDate && sleepDay && <DailySleepEditor open date={sleepDate} wake={sleepDay.wakeAt || data.user?.wake_up_time || "07:00"} bedtime={sleepDay.bedtimeAt || data.user?.bedtime || "23:00"} timezone={data.timezone} revision={sleepDay.sleepRevision} hasException={sleepDay.hasSleepException} busy={actions.busy} onSave={saveSleep} onReset={resetSleep} onCancel={() => setSleepDate(undefined)} />}
  </>
}
