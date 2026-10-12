import { useContext, useState } from "react"
import { useQuery } from "@tanstack/react-query"
import type { RestInboxItemResponse } from "~/api/generated/inbox"
import { getGetTasksTaskIdActionItemsQueryKey } from "~/api/generated/tasks"
import { getGetTodayTodoQueryOptions } from "~/api/generated/todo"
import { useI18n, useLanguage } from "~/features/i18n/hooks"
import { InboxActionContext } from "~/features/inbox/providers/InboxActionProvider/context"
import type { InboxActions } from "~/features/inbox/providers/InboxActionProvider/context"
import { InboxAPIDataContext } from "~/features/inbox/providers/InboxAPIDataProvider/context"
import { InboxNavigationContext } from "~/features/inbox/providers/InboxNavigationProvider/context"
import controls from "~/styles/controls.module.css"
import { formatInboxSchedule } from "./formatInboxSchedule"
import { getAllTaskActionItems } from "~/features/task/queries"
import { todoItems2registrationTimeLabels } from "~/features/todo/converters/todoItems2registrationTimeLabels"
import type { RestActionItemResponse } from "~/api/generated/tasks"
import ActionItemActualMinutesDialog from "~/features/shared/components/ActionItemActualMinutesDialog"
import styles from "./index.module.css"

function formatLocalDate(value: string, locale: string) {
  const date = new Date(`${value}T12:00:00Z`)
  return new Intl.DateTimeFormat(locale, { month: "numeric", day: "numeric", weekday: "short", timeZone: "UTC" }).format(date)
}

export default function InboxList() {
  const i18n = useI18n()
  const { language } = useLanguage()
  const { inbox, timezone } = useContext(InboxAPIDataContext)!
  const { busy, complete } = useContext(InboxActionContext)!
  const { editItem, state } = useContext(InboxNavigationContext)!
  const todayQuery = useQuery({ ...getGetTodayTodoQueryOptions(), retry: false })
  const registrationTimeLabels = todoItems2registrationTimeLabels(todayQuery.data?.items, todayQuery.data?.timezone || timezone || "UTC", language === "ja" ? "ja-JP" : "en-US")
  if (!inbox) return null
  if (inbox.items.length === 0) return state.pageToken ? null : <p className={`${styles.empty} text-body`}>{i18n("inbox.empty")}</p>

  return <div className={styles.list} role="list" aria-label={i18n("inbox.list.title")}>
    {inbox.items.map((item) => {
      if (item.type === "task") return <InboxTaskRow key={`task-${item.id}`} item={item} registrationTimeLabels={registrationTimeLabels} />
      const canComplete = Boolean(item.schedule?.can_update)
      const canEdit = Boolean(item.schedule?.can_update || item.schedule?.can_update_following)
      const locale = language === "ja" ? "ja-JP" : "en-US"
      const details = [formatInboxSchedule(item.schedule!, timezone, locale, i18n("inbox.row.allDay"))]
      return <article className={styles.row} role="listitem" key={`${item.type}-${item.id}`}>
        <span className={styles.kindIcon} aria-hidden="true"><CalendarIcon /></span>
        <div className={styles.itemMain}>
          <h2 className={`${styles.title} text-body-strong`}>{item.title}</h2>
          <div className={styles.itemMeta}>
            <span className="text-caption">{item.project_name || i18n("inbox.row.noProject")}</span>
            <span className="text-caption">{i18n(`inbox.row.${item.type}`)}</span>
            {details.map((detail) => <span className="text-caption" key={detail}>{detail}</span>)}
          </div>
        </div>
        <div className={styles.actions}>
        {item.type === "schedule" && <CompletionButton item={item} disabled={busy || !canComplete} complete={complete} />}
          {canEdit && <button className={`${controls.button} ${controls.neutralButton} ${controls.focusRing} ${styles.editButton} text-button-small`} type="button" disabled={busy} aria-label={i18n("inbox.row.edit", { title: item.title })} onClick={() => editItem(item)}>{i18n("inbox.row.editLabel")}</button>}
        </div>
      </article>
    })}
  </div>
}

function InboxTaskRow({ item, registrationTimeLabels }: { item: RestInboxItemResponse; registrationTimeLabels: Map<string, string> }) {
  const i18n = useI18n()
  const { language } = useLanguage()
  const { busy, complete, completeActionItem, reopenActionItem, addActionItemToToday, addTaskToToday } = useContext(InboxActionContext)!
  const { editItem } = useContext(InboxNavigationContext)!
  const taskId = item.task?.id ?? item.id
  const taskTitle = item.title
  const canEdit = Boolean(item.task?.can_update)
  const canComplete = canEdit
  const [expanded, setExpanded] = useState(false)
  const [pendingCompletion, setPendingCompletion] = useState<RestActionItemResponse | null>(null)
  const [pendingTaskCompletion, setPendingTaskCompletion] = useState(false)
  const panelId = `inbox-action-items-${taskId}`
  const query = useQuery({
    queryKey: [...getGetTasksTaskIdActionItemsQueryKey(taskId), "all-pages"],
    queryFn: () => getAllTaskActionItems(taskId),
    enabled: expanded,
    retry: false,
  })
  return <div className={styles.taskRowGroup} role="listitem">
    <article className={styles.row}>
      <button className={`${controls.focusRing} ${styles.completeButton} ${styles.taskComplete}`} type="button" disabled={busy || !canComplete} title={!canComplete ? i18n("inbox.row.noPermission") : undefined} aria-label={i18n("inbox.row.complete", { title: taskTitle })} onClick={() => {
        if (item.task?.action_item_count === 0) setPendingTaskCompletion(true)
        else void complete(item).catch(() => undefined)
      }}><span aria-hidden="true">✓</span></button>
      <div className={styles.itemMain}>
        <h2 className={`${styles.title} text-body-strong`}>{taskTitle}</h2>
        <div className={styles.itemMeta}>
          <span className="text-caption">{item.project_name || i18n("inbox.row.noProject")}</span>
          <span className="text-caption">{i18n("inbox.row.task")}</span>
          {item.task?.estimated_minutes != null && <span className="text-caption">{i18n("inbox.detail.estimate", { hours: Math.floor(item.task.estimated_minutes / 60), minutes: item.task.estimated_minutes % 60 })}</span>}
          {item.task?.due_date && <span className="text-caption">{i18n("inbox.detail.due", { date: formatLocalDate(item.task.due_date, language === "ja" ? "ja-JP" : "en-US") })}</span>}
        </div>
      </div>
      <div className={styles.actions}>
        {canEdit && <button className={`${controls.button} ${controls.neutralButton} ${controls.focusRing} ${styles.editButton} text-button-small`} type="button" disabled={busy} aria-label={i18n("inbox.row.edit", { title: taskTitle })} onClick={() => editItem(item)}>{i18n("inbox.row.editLabel")}</button>}
        {item.task?.action_item_count === 0 && item.task.can_add_to_today && !registrationTimeLabels.has(taskId) && <button className={`${controls.button} ${controls.outlineButton} ${controls.focusRing} ${styles.todayButton} text-button-small`} type="button" disabled={busy} onClick={() => void addTaskToToday(taskId, item.task?.priority).catch(() => undefined)}>{i18n("tasks.actionItem.registerToday")}</button>}
        {item.task?.action_item_count === 0 && registrationTimeLabels.has(taskId) && <span className="text-caption">{i18n("tasks.actionItem.scheduled", { time: registrationTimeLabels.get(taskId) ?? "—" })}</span>}
        <button type="button" className={`${controls.button} ${controls.outlineButton} ${controls.focusRing} ${styles.todayButton} text-button-small`} aria-controls={panelId} aria-expanded={expanded} aria-label={i18n(expanded ? "inbox.row.collapseActionItems" : "inbox.row.expandActionItems", { title: taskTitle })} onClick={() => setExpanded((value) => !value)}>
      {i18n(expanded ? "inbox.row.hideActionItems" : "inbox.row.showActionItems")}
        </button>
      </div>
    </article>
    <section className={styles.actionItems} id={panelId} hidden={!expanded} aria-label={i18n("inbox.row.actionItemsFor", { title: taskTitle })}>
      {query.isPending && <p className="text-caption" role="status">{i18n("inbox.row.actionItemsLoading")}</p>}
      {query.isError && <div className={styles.actionItemsError} role="alert">
        <span className="text-caption">{i18n("inbox.row.actionItemsError")}</span>
        <button type="button" className={styles.retryLink} onClick={() => void query.refetch()}>{i18n("inbox.retry")}</button>
      </div>}
      {query.data?.length === 0 && <p className="text-caption">{i18n("inbox.row.noActionItems")}</p>}
      {query.data && query.data.length > 0 && <ul className={styles.actionItemsList}>
        {query.data.map((actionItem) => <li className={styles.actionItem} key={actionItem.id}>
          <button className={styles.actionItemComplete} type="button" disabled={busy || !canEdit || !actionItem.id} title={!canEdit ? i18n("inbox.row.noPermission") : undefined} aria-label={i18n(actionItem.completed ? "inbox.row.reopen" : "inbox.row.complete", { title: actionItem.title ?? "" })} aria-pressed={Boolean(actionItem.completed)} onClick={() => actionItem.completed ? void reopenActionItem(taskId, actionItem).catch(() => undefined) : setPendingCompletion(actionItem)}>
            <span aria-hidden="true">{actionItem.completed ? "✓" : ""}</span>
          </button>
          <span className={`text-body ${actionItem.completed ? styles.actionItemCompleted : ""}`}>{actionItem.title}</span>
          <span className="text-caption">{actionItem.estimated_minutes == null ? "—" : i18n("inbox.detail.estimate", { hours: Math.floor(actionItem.estimated_minutes / 60), minutes: actionItem.estimated_minutes % 60 })}</span>
          <span className="text-caption" aria-label={actionItem.id && registrationTimeLabels.has(actionItem.id) ? i18n("tasks.actionItem.scheduled", { time: registrationTimeLabels.get(actionItem.id) ?? "—" }) : undefined}>{actionItem.id ? registrationTimeLabels.get(actionItem.id) ?? "—" : "—"}</span>
          {!actionItem.completed && <button className={`${controls.button} ${controls.outlineButton} ${controls.focusRing} text-button-small`} type="button" disabled={busy || !canEdit || !actionItem.id} title={!canEdit ? i18n("inbox.row.noPermission") : undefined} onClick={() => void addActionItemToToday(taskId, actionItem).catch(() => undefined)}>{i18n("tasks.actionItem.registerToday")}</button>}
        </li>)}
      </ul>}
    </section>
    <ActionItemActualMinutesDialog open={Boolean(pendingCompletion)} busy={busy} onCancel={() => setPendingCompletion(null)} onComplete={(actualMinutes) => {
      if (!pendingCompletion) return
      void completeActionItem(taskId, pendingCompletion, actualMinutes).then(() => setPendingCompletion(null)).catch(() => undefined)
    }} />
    <ActionItemActualMinutesDialog open={pendingTaskCompletion} busy={busy} onCancel={() => setPendingTaskCompletion(false)} onComplete={(actualMinutes) => {
      void complete(item, actualMinutes).then(() => setPendingTaskCompletion(false)).catch(() => undefined)
    }} />
  </div>
}

function CompletionButton({ item, disabled, complete }: { item: RestInboxItemResponse; disabled: boolean; complete: InboxActions["complete"] }) {
  const i18n = useI18n()
  return <button
    className={`${controls.focusRing} ${styles.completeButton} ${styles.scheduleComplete}`}
    type="button"
    disabled={disabled}
    title={disabled ? i18n("inbox.row.noPermission") : undefined}
    aria-label={i18n("inbox.row.complete", { title: item.title })}
    onClick={() => void complete(item).catch(() => undefined)}
  ><span aria-hidden="true">✓</span></button>
}

function CalendarIcon() {
  return <svg viewBox="0 0 24 24" width="22" height="22" fill="none" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" strokeLinejoin="round"><rect x="3.5" y="5" width="17" height="16" rx="2" /><path d="M8 3v4M16 3v4M4 10h16" /></svg>
}
