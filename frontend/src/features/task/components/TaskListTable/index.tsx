import { useI18n } from "~/features/i18n/hooks"
import controls from "~/styles/controls.module.css"
import styles from "./index.module.css"
import type { TaskActionItemRowView, TaskActionItemsQueryState, TaskListRowView } from "./types"
import { useTaskListTable } from "./useTaskListTable"

export default function TaskListTable() {
  const i18n = useI18n()
  const { rows, summary, expandedIds, busyActionItemKey, actionItemsQueryStates, loading, error, toggleExpanded, toggleActionItem, retryActionItems, retry, edit } = useTaskListTable()
  const expanded = new Set(expandedIds)
  if (loading) return <p className={`${styles.listMessage} text-body`} role="status">{i18n("tasks.loading")}</p>
  if (error) return (
    <div className={styles.listMessage} role="alert">
      <p className="text-body">{i18n("tasks.loadError")}</p>
      <button className={`${controls.button} ${controls.outlineButton} ${controls.focusRing} text-button-small`} type="button" onClick={retry}>{i18n("tasks.retry")}</button>
    </div>
  )
  if (rows.length === 0) return <p className={`${styles.listMessage} text-body`}>{i18n("tasks.empty")}</p>
  return (
    <div className={styles.scroll} role="region" aria-label={i18n("page.tasks.title")} tabIndex={0}>
      <table className={styles.table}>
        <caption className={styles.srOnly}>{i18n("page.tasks.title")}</caption>
        <thead>
          <tr>
            <th scope="col">{i18n("tasks.column.task")}</th>
            <th scope="col">{i18n("tasks.column.project")}</th>
            <th scope="col">{i18n("tasks.column.status")}</th>
            <th scope="col">{i18n("tasks.column.actionItems")}</th>
            <th scope="col" className={styles.numeric}>{i18n("tasks.column.estimate")}</th>
            <th scope="col">{i18n("tasks.column.deadline")}</th>
            <th scope="col">{i18n("tasks.column.actions")}</th>
          </tr>
        </thead>
        <tbody>
          {rows.map((task) => {
            const isExpanded = expanded.has(task.id)
            return (
              <TaskRows
                key={task.id}
                task={task}
                expanded={isExpanded}
                busyActionItemKey={busyActionItemKey}
                actionItemsQueryState={actionItemsQueryStates[task.id]}
                onToggleExpanded={toggleExpanded}
                onToggleActionItem={toggleActionItem}
                onRetryActionItems={retryActionItems}
                onEdit={edit}
              />
            )
          })}
        </tbody>
        <tfoot>
          <tr>
            <th scope="row"><span className={styles.totalHeading}><span>{i18n("tasks.summary.total")}</span><span className="text-caption">{summary.totalLabel}</span></span></th>
            <td />
            <td />
            <td>
              <div className={styles.progressCell}>
                <ProgressBar completedLabel={summary.completedActionItemsLabel} totalLabel={summary.totalActionItemsLabel} progress={summary.totalActionItems > 0 ? summary.completedActionItems / summary.totalActionItems * 100 : 0} />
                <span className="text-numeric">{summary.completedActionItemsLabel} / {summary.totalActionItemsLabel}</span>
              </div>
            </td>
            <td className={`${styles.numeric} text-numeric`}>{summary.estimateTotalLabel}</td>
            <td />
            <td />
          </tr>
        </tfoot>
      </table>
    </div>
  )
}

function TaskRows({ task, expanded, busyActionItemKey, actionItemsQueryState, onToggleExpanded, onToggleActionItem, onRetryActionItems, onEdit }: {
  task: TaskListRowView
  expanded: boolean
  busyActionItemKey?: string
  actionItemsQueryState?: TaskActionItemsQueryState
  onToggleExpanded: (taskId: string) => void
  onToggleActionItem: (item: TaskActionItemRowView) => void
  onRetryActionItems: (taskId: string) => void
  onEdit: (taskId: string) => void
}) {
  const i18n = useI18n()
  return (
    <>
      <tr className={task.statusTone === "done" ? styles.doneTask : undefined}>
        <td>
          <div className={styles.taskCell}>
            {task.actionItemCount > 0 ? (
              <button className={styles.expandButton} type="button" aria-label={i18n(expanded ? "tasks.actionItem.collapse" : "tasks.actionItem.expand", { title: task.title })} aria-expanded={expanded} onClick={() => onToggleExpanded(task.id)}>
                <svg aria-hidden="true" viewBox="0 0 16 16" width="16" height="16" fill="none">
                  {expanded ? <path d="m4 6 4 4 4-4" /> : <path d="m6 4 4 4-4 4" />}
                </svg>
              </button>
            ) : <span className={styles.expandSpacer} aria-hidden="true" />}
            <strong className="text-body-strong">{task.title}</strong>
          </div>
        </td>
        <td>{task.projectLabel}</td>
        <td><span className={`${styles.status} ${styles[`status_${task.statusTone}`]}`}><span className={styles.statusMark} aria-hidden="true">{task.statusTone === "active" || task.statusTone === "done" ? "●" : "○"}</span>{task.statusLabel}</span></td>
        <td>
          <div className={styles.progressCell}>
            <ProgressBar completedLabel={String(task.actionItemCompletedCount)} totalLabel={String(task.actionItemCount)} progress={task.progress} />
            <span className="text-numeric">{task.actionItemProgressLabel}</span>
          </div>
        </td>
        <td className={`${styles.numeric} text-numeric`}>{task.estimateLabel}</td>
        <td><span className={styles[`due_${task.dueDateTone}`]}>{task.dueDateLabel}</span></td>
        <td>{task.canUpdate && <button className={`${styles.editButton} text-button-small`} type="button" aria-label={i18n("tasks.editTask", { title: task.title })} onClick={() => onEdit(task.id)}>{i18n("tasks.edit")}</button>}</td>
      </tr>
      {expanded && task.actionItemCount > 0 && (
        <tr className={styles.expandedRow}>
          <td colSpan={7}>
            {actionItemsQueryState === "loading" ? <p className={`${styles.detailsMessage} text-body`} role="status">{i18n("tasks.actionItem.loading")}</p> : actionItemsQueryState === "error" ? (
              <div className={styles.detailsMessage} role="alert">
                <p className="text-body">{i18n("tasks.actionItem.loadError")}</p>
                <button className="text-button-small" type="button" onClick={() => onRetryActionItems(task.id)}>{i18n("tasks.retry")}</button>
              </div>
            ) : task.actionItems.length === 0 ? <p className={`${styles.detailsMessage} text-body`}>{i18n("tasks.actionItem.none")}</p> : (
              <div className={styles.actionItems}>
                {task.actionItems.map((item) => (
                  <ActionItemRow key={item.occurrenceKey} item={item} busy={busyActionItemKey === item.occurrenceKey} onToggle={onToggleActionItem} />
                ))}
              </div>
            )}
          </td>
        </tr>
      )}
    </>
  )
}

function ActionItemRow({ item, busy, onToggle }: { item: TaskActionItemRowView; busy: boolean; onToggle: (item: TaskActionItemRowView) => void }) {
  const i18n = useI18n()
  return (
    <div className={`${styles.actionItem} ${item.completed ? styles.completedActionItem : ""}`}>
      <button className={styles.checkButton} type="button" disabled={busy} aria-label={i18n(item.completed ? "tasks.actionItem.reopen" : "tasks.actionItem.complete", { title: item.title })} aria-pressed={item.completed} onClick={() => onToggle(item)}>
        <span aria-hidden="true">{item.completed ? "✓" : ""}</span>
      </button>
      <span className={styles.itemTitle}><span className="text-body">{item.title}</span>{item.occurrenceDateLabel && <span className={`${styles.occurrenceDate} text-caption`}>{item.occurrenceDateLabel}</span>}</span>
      <span className={`${styles.itemEstimate} text-numeric`}>{item.estimateLabel}</span>
      {item.todayRegistration ? (
        <RegisteredTime timeLabel={item.todayRegistration.timeLabel} />
      ) : (
        <button className={`${styles.registerButton} text-button-small`} type="button" disabled aria-disabled="true">{i18n("tasks.actionItem.registerToday")}</button>
      )}
    </div>
  )
}

export function RegisteredTime({ timeLabel }: { timeLabel: string }) {
  const i18n = useI18n()
  return <span className={`${styles.registeredTime} text-caption`}><span aria-hidden="true">◷</span>{i18n("tasks.actionItem.scheduled", { time: timeLabel })}</span>
}

function ProgressBar({ completedLabel, totalLabel, progress }: { completedLabel: string; totalLabel: string; progress: number }) {
  const i18n = useI18n()
  return (
    <div className={styles.progressTrack} role="progressbar" aria-label={i18n("tasks.actionItem.progress")} aria-valuemin={0} aria-valuemax={100} aria-valuenow={Math.min(100, Math.max(0, progress))} aria-valuetext={`${completedLabel} / ${totalLabel}`}>
      <span className={styles.progressFill} style={{ width: `${Math.min(100, Math.max(0, progress))}%` }} />
    </div>
  )
}
