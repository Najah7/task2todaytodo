import { useContext, useEffect, useMemo, useState } from "react"
import type { RestTodoItemResponse } from "~/api/generated/todo"
import type { MessageKey } from "~/features/i18n/messages/types"
import { useI18n, useLanguage } from "~/features/i18n/hooks"
import ActionItemActualMinutesDialog from "~/features/shared/components/ActionItemActualMinutesDialog"
import ConfirmationDialog from "~/features/shared/components/ConfirmationDialog"
import TodayBoard from "~/features/todo/components/TodayBoard"
import { civilDateLabel, minutesLabel, timeLabel, todoItems2boardItems } from "~/features/todo/converters/todoItems2boardItems"
import { TodayActionProvider } from "~/features/todo/providers/TodayActionProvider"
import { TodayActionContext } from "~/features/todo/providers/TodayActionProvider/context"
import { useGetBillingStatus } from "~/api/generated/billing"
import { TodayAPIDataProvider } from "~/features/todo/providers/TodayAPIDataProvider"
import { TodayAPIDataContext } from "~/features/todo/providers/TodayAPIDataProvider/context"
import controls from "~/styles/controls.module.css"
import styles from "./index.module.css"

function TodayContent() {
  const data = useContext(TodayAPIDataContext)
  const actions = useContext(TodayActionContext)
  const i18n = useI18n()
  const billing = useGetBillingStatus()
  const { language } = useLanguage()
  const [showHidden, setShowHidden] = useState(false)
  const [showHelp, setShowHelp] = useState(false)
  const [confirmRegenerate, setConfirmRegenerate] = useState(false)
  const [withoutAllocations, setWithoutAllocations] = useState(false)
  const [pendingRestore, setPendingRestore] = useState<RestTodoItemResponse>()
  const [pendingComplete, setPendingComplete] = useState<RestTodoItemResponse>()
  const [now, setNow] = useState(() => Date.now())

  useEffect(() => {
    const timer = window.setInterval(() => setNow(Date.now()), 30_000)
    return () => window.clearInterval(timer)
  }, [])

  const todo = data?.todo
  const timezone = todo?.timezone || data?.timezone || "UTC"
  const locale = language === "ja" ? "ja-JP" : "en-US"
  const items = useMemo(() => todoItems2boardItems(todo?.items || [], timezone, locale), [locale, timezone, todo?.items])
  const visibleItems = items.filter((item) => !item.history)
  const reorderIds = visibleItems.map((item) => item.id)
  const completedCount = todo?.progress?.completed_items || 0
  const totalCount = todo?.progress?.total_items || 0
  const next = todo?.next_schedule
  const nextStart = next?.scheduled_start ? new Date(next.scheduled_start).getTime() : undefined
  const countdownMinutes = nextStart === undefined ? undefined : Math.max(0, Math.floor((nextStart - now) / 60_000))
  const priorityPolicy = Object.entries(todo?.policy?.priority_weights || {}).map(([key, value]) => ({ label: i18n(`today.help.priority.${key}` as MessageKey), value }))
  const urgencyPolicy = [
    ["sameOrPast", todo?.policy?.urgency_same_or_past],
    ["days1to7", todo?.policy?.urgency_1_to_7],
    ["days8to14", todo?.policy?.urgency_8_to_14],
    ["days15to30", todo?.policy?.urgency_15_to_30],
    ["days31plus", todo?.policy?.urgency_31_plus],
    ["none", todo?.policy?.urgency_none],
  ].filter((entry): entry is [string, number] => entry[1] !== undefined).map(([key, value]) => ({ label: i18n(`today.help.due.${key}` as MessageKey), value }))

  if (!data || data.state === "loading") return <p className="text-body">{i18n("inbox.loading")}</p>
  if (data.state === "error" || !todo || !actions) return <div className={styles.loadError}><p className="text-body">{i18n("today.error")}</p><button className={`${controls.button} ${controls.outlineButton} ${controls.focusRing} text-button-small`} type="button" onClick={() => void data.refetch()}>{i18n("inbox.retry")}</button></div>
  const todayActions = actions
  const canRegenerateWithoutAllocations = billing.data?.access_plan === "pro" || billing.data?.access_plan === "team"

  async function complete(itemId: string, actualMinutes?: number) {
    const success = await todayActions.complete(itemId, actualMinutes)
    if (success) setPendingComplete(undefined)
  }

  function move(itemId: string, direction: -1 | 1) {
    const index = reorderIds.indexOf(itemId)
    const target = index + direction
    if (index < 0 || target < 0 || target >= reorderIds.length) return
    const nextIds = [...reorderIds]
    ;[nextIds[index], nextIds[target]] = [nextIds[target], nextIds[index]]
    void todayActions.reorder(nextIds)
  }

  function reorder(itemId: string, targetItemId: string, position: "before" | "after") {
    const nextIds = reorderIds.filter((id) => id !== itemId)
    const targetIndex = nextIds.indexOf(targetItemId)
    if (targetIndex < 0) return
    nextIds.splice(targetIndex + (position === "after" ? 1 : 0), 0, itemId)
    void todayActions.reorder(nextIds)
  }

  return <>
    <TodayBoard
      dateLabel={civilDateLabel(todo.date, locale)}
      items={items}
      completedCount={completedCount}
      totalCount={totalCount}
      totalTimeLabel={minutesLabel(todo.progress?.total_minutes)}
      completedTimeLabel={minutesLabel(todo.progress?.completed_minutes)}
      remainingTimeLabel={minutesLabel(todo.progress?.remaining_minutes)}
      nextSchedule={next ? { title: next.title || "", startLabel: timeLabel(next.scheduled_start, timezone, locale), endLabel: timeLabel(next.scheduled_end, timezone, locale), countdownLabel: countdownMinutes === undefined ? "—" : minutesLabel(countdownMinutes) } : undefined}
      hiddenScheduleCount={todo.hidden_schedules?.length || 0}
      busy={todayActions.busy}
      onComplete={(itemId) => {
        const item = todo.items?.find((candidate) => candidate.id === itemId)
        if (item?.kind === "task" || item?.kind === "action_item") setPendingComplete(item)
        else void todayActions.complete(itemId)
      }}
      onReopen={(itemId) => void todayActions.reopen(itemId)}
      onMove={move}
      onReorder={reorder}
      onRemove={(itemId) => void todayActions.remove(itemId)}
      onHideSchedule={(itemId) => void todayActions.hide(itemId)}
      onShowHiddenSchedules={() => setShowHidden(true)}
      onOpenHelp={() => setShowHelp(true)}
      onRegenerate={() => { setWithoutAllocations(false); setConfirmRegenerate(true) }}
      onRegenerateWithoutAllocations={canRegenerateWithoutAllocations ? () => { setWithoutAllocations(true); setConfirmRegenerate(true) } : undefined}
    />
    <ActionItemActualMinutesDialog open={Boolean(pendingComplete)} busy={todayActions.busy} onComplete={(minutes) => pendingComplete?.id && void complete(pendingComplete.id, minutes)} onCancel={() => setPendingComplete(undefined)} />
    <ConfirmationDialog open={confirmRegenerate} title={i18n(withoutAllocations ? "today.confirm.regenerateWithoutAllocationsTitle" : "today.confirm.regenerateTitle")} description={i18n(withoutAllocations ? "today.confirm.regenerateWithoutAllocationsDescription" : "today.confirm.regenerateDescription")} confirmLabel={i18n("today.confirm.regenerate")} cancelLabel={i18n("today.confirm.cancel")} destructive busy={todayActions.busy} onCancel={() => setConfirmRegenerate(false)} onConfirm={() => { void todayActions.regenerate(withoutAllocations).then((ok) => { if (ok) setConfirmRegenerate(false) }) }} />
    <ConfirmationDialog open={Boolean(pendingRestore)} title={i18n("today.confirm.restoreTitle")} description={i18n("today.confirm.restoreDescription")} confirmLabel={i18n("today.confirm.restore")} cancelLabel={i18n("today.confirm.cancel")} busy={todayActions.busy} onCancel={() => setPendingRestore(undefined)} onConfirm={() => { const item = pendingRestore; if (!item?.id) return; void todayActions.restore(item.id).then((ok) => { if (ok) { setPendingRestore(undefined); if ((todo.hidden_schedules?.length || 0) <= 1) setShowHidden(false) } }) }} />
    {showHidden && <dialog className={styles.dialog} open aria-labelledby="today-hidden-title">
      <h2 className="text-card-title" id="today-hidden-title">{i18n("today.hidden.title")}</h2>
      <ul className={styles.hiddenList}>{(todo.hidden_schedules || []).map((item) => <li key={item.id}><span className="text-body-strong">{item.title}</span><span className="text-caption">{timeLabel(item.scheduled_start, timezone, locale)} – {timeLabel(item.scheduled_end, timezone, locale)}</span><button className={`${controls.button} ${controls.outlineButton} ${controls.focusRing} text-button-small`} type="button" disabled={todayActions.busy} onClick={() => setPendingRestore(item)}>{i18n("today.hidden.restore")}</button></li>)}</ul>
      <button className={`${controls.button} ${controls.neutralButton} ${controls.focusRing} text-button-small`} type="button" onClick={() => setShowHidden(false)}>{i18n("today.hidden.close")}</button>
    </dialog>}
    {showHelp && <dialog className={styles.dialog} open aria-labelledby="today-help-title">
      <h2 className="text-card-title" id="today-help-title">{i18n("today.help.title")}</h2>
      <p className="text-body">{i18n("today.help.description")}</p>
      <ol className={`${styles.helpSteps} text-body`}>
        <li>{i18n("today.help.fixed")}</li>
        <li>{i18n("today.help.available")}</li>
        <li>{i18n("today.help.order")}</li>
        <li>{i18n("today.help.score")}</li>
      </ol>
      <section className={styles.policyCard} aria-labelledby="today-help-priority-title">
        <h3 className="text-label" id="today-help-priority-title">{i18n("today.help.priorityHeading")}</h3>
        <dl>{priorityPolicy.map(({ label, value }) => <div key={label}><dt className="text-caption">{label}</dt><dd className="text-numeric">{value}</dd></div>)}</dl>
      </section>
      <section className={styles.policyCard} aria-labelledby="today-help-urgency-title">
        <h3 className="text-label" id="today-help-urgency-title">{i18n("today.help.urgencyHeading")}</h3>
        <dl>{urgencyPolicy.map(({ label, value }) => <div key={label}><dt className="text-caption">{label}</dt><dd className="text-numeric">{value}</dd></div>)}</dl>
      </section>
      <p className="text-body">{i18n("today.help.manual")}</p>
      <p className="text-body">{i18n("today.help.allDayHistory")}</p>
      <button className={`${controls.button} ${controls.primaryButton} ${controls.focusRing} text-button-small`} type="button" onClick={() => setShowHelp(false)}>{i18n("today.hidden.close")}</button>
    </dialog>}
  </>
}

export default function TodayPage() {
  return <TodayAPIDataProvider><TodayActionProvider><TodayContent /></TodayActionProvider></TodayAPIDataProvider>
}
