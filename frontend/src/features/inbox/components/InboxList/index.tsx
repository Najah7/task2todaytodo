import { useContext } from "react"
import type { RestInboxItemResponse } from "~/api/generated/inbox"
import { useI18n, useLanguage } from "~/features/i18n/hooks"
import { InboxActionContext } from "~/features/inbox/providers/InboxActionProvider/context"
import type { InboxActions } from "~/features/inbox/providers/InboxActionProvider/context"
import { InboxAPIDataContext } from "~/features/inbox/providers/InboxAPIDataProvider/context"
import { InboxNavigationContext } from "~/features/inbox/providers/InboxNavigationProvider/context"
import controls from "~/styles/controls.module.css"
import { formatInboxSchedule } from "./formatInboxSchedule"
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
  if (!inbox) return null
  if (inbox.items.length === 0) return state.pageToken ? null : <p className={`${styles.empty} text-body`}>{i18n("inbox.empty")}</p>

  return <div className={styles.list} role="list" aria-label={i18n("inbox.list.title")}>
    {inbox.items.map((item) => {
      const canComplete = item.type === "task" ? Boolean(item.task?.can_update) : Boolean(item.schedule?.can_update)
      const canEdit = item.type === "task" ? Boolean(item.task?.can_update) : Boolean(item.schedule?.can_update || item.schedule?.can_update_following)
      const locale = language === "ja" ? "ja-JP" : "en-US"
      const details = item.type === "schedule"
        ? [formatInboxSchedule(item.schedule!, timezone, locale, i18n("inbox.row.allDay"))]
        : [
          ...(item.task?.estimated_minutes != null ? [i18n("inbox.detail.estimate", { hours: Math.floor(item.task.estimated_minutes / 60), minutes: item.task.estimated_minutes % 60 })] : []),
          ...(item.task?.due_date ? [i18n("inbox.detail.due", { date: formatLocalDate(item.task.due_date, locale) })] : []),
        ]
      return <article className={styles.row} role="listitem" key={`${item.type}-${item.id}`}>
        {item.type === "schedule"
          ? <span className={styles.kindIcon} aria-hidden="true"><CalendarIcon /></span>
          : <CompletionButton item={item} disabled={busy || !canComplete} complete={complete} variant="task" />}
        <div className={styles.itemMain}>
          <h2 className={`${styles.title} text-body-strong`}>{item.title}</h2>
          <div className={styles.itemMeta}>
            <span className="text-caption">{item.project_name || i18n("inbox.row.noProject")}</span>
            <span className="text-caption">{i18n(`inbox.row.${item.type}`)}</span>
            {details.map((detail) => <span className="text-caption" key={detail}>{detail}</span>)}
          </div>
        </div>
        <div className={styles.actions}>
          {item.type === "schedule" && <CompletionButton item={item} disabled={busy || !canComplete} complete={complete} variant="schedule" />}
          {canEdit && <button className={`${controls.button} ${controls.neutralButton} ${controls.focusRing} ${styles.editButton} text-button-small`} type="button" disabled={busy} aria-label={i18n("inbox.row.edit", { title: item.title })} onClick={() => editItem(item)}>{i18n("inbox.row.editLabel")}</button>}
          {item.type === "task" && <button className={`${controls.button} ${controls.outlineButton} ${controls.focusRing} ${styles.todayButton} text-button-small`} type="button" disabled title={i18n("inbox.row.noTodayApi")} aria-label={i18n("inbox.row.addToday")}>{i18n("inbox.row.addToday")}</button>}
        </div>
      </article>
    })}
  </div>
}

function CompletionButton({ item, disabled, complete, variant }: { item: RestInboxItemResponse; disabled: boolean; complete: InboxActions["complete"]; variant: "task" | "schedule" }) {
  const i18n = useI18n()
  return <button
    className={`${controls.focusRing} ${styles.completeButton} ${variant === "task" ? styles.taskComplete : styles.scheduleComplete}`}
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
