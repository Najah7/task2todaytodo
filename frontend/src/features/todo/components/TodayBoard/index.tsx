import { useState, type DragEvent } from "react"
import { useI18n } from "~/features/i18n/hooks"
import controls from "~/styles/controls.module.css"
import styles from "./index.module.css"
import type { TodayBoardItem, TodayBoardProps } from "./types"

export default function TodayBoard(props: TodayBoardProps) {
  const i18n = useI18n()
  const progress = props.totalCount > 0 ? props.completedCount / props.totalCount * 100 : 0
  const [draggedId, setDraggedId] = useState<string>()
  const orderedItems = props.items.filter((item) => !item.history)

  function dropOn(item: TodayBoardItem, event: DragEvent<HTMLElement>) {
    if (!draggedId || draggedId === item.id || item.history) return
    const rect = event.currentTarget.getBoundingClientRect()
    const position = event.clientY < rect.top + rect.height / 2 ? "before" : "after"
    props.onReorder(draggedId, item.id, position)
    setDraggedId(undefined)
  }

  return <section className={styles.page} aria-labelledby="today-board-title">
    <header className={styles.header}>
      <div>
        <h1 className="text-page-title" id="today-board-title">{i18n("page.today.title")}</h1>
        <p className="text-description">{props.dateLabel}</p>
      </div>
      <div className={styles.headerActions}>
        <button className={`${controls.button} ${controls.outlineButton} ${controls.focusRing} text-button-small`} type="button" disabled={props.busy} onClick={props.onRegenerate}>{i18n("today.regenerate")}</button>
        <button className={`${controls.button} ${controls.neutralButton} ${controls.focusRing} text-button-small`} type="button" onClick={props.onOpenHelp}>{i18n("today.helpLink")}</button>
      </div>
    </header>
    <div className={styles.columns}>
      <section className={`${controls.card} ${styles.taskCard}`} aria-label={i18n("today.items.title")}>
        <div className={styles.cardHeading}><h2 className="text-section-title">{i18n("today.items.title")}</h2></div>
        {props.items.length === 0 ? <p className={`${styles.empty} text-body`}>{i18n("today.items.empty")}</p> : <div className={styles.itemList} role="list">
          <div className={`${styles.itemHeader} text-label`} aria-hidden="true"><span>{i18n("today.column.status")}</span><span>{i18n("today.column.time")}</span><span>{i18n("today.column.task")}</span><span>{i18n("today.column.estimate")}</span><span>{i18n("today.column.actions")}</span></div>
          {props.items.map((item) => <article
            className={`${styles.item} ${item.history ? styles.history : ""} ${item.completed ? styles.completed : ""} ${draggedId === item.id ? styles.dragging : ""}`}
            key={item.id}
            role="listitem"
            draggable={!props.busy && !item.history && item.canMove}
            onDragStart={(event) => {
              if (!item.canMove) event.preventDefault()
              else setDraggedId(item.id)
            }}
            onDragEnd={() => setDraggedId(undefined)}
            onDragOver={(event) => {
              if (draggedId && draggedId !== item.id && !item.history) event.preventDefault()
            }}
            onDrop={(event) => { event.preventDefault(); dropOn(item, event) }}
          >
            <div className={styles.statusCell}>
              {item.canComplete && <button className={`${styles.checkButton} ${controls.focusRing}`} type="button" aria-label={i18n("today.item.complete", { title: item.title })} disabled={props.busy} onClick={() => props.onComplete(item.id)}>✓</button>}
              {item.canReopen && <button className={`${styles.checkButton} ${styles.checkedButton} ${controls.focusRing}`} type="button" aria-label={i18n("today.item.reopen", { title: item.title })} disabled={props.busy} onClick={() => props.onReopen(item.id)}>✓</button>}
              {!item.canComplete && !item.canReopen && <span className={`${styles.statusPill} text-caption`}>{i18n(item.status)}</span>}
            </div>
            <span className={`${styles.time} text-numeric`}>{item.timeLabel}</span>
            <div className={styles.titleCell}>
              <strong className="text-body-strong">{item.title}</strong>
              <span className="text-caption">{item.groupLabel}</span>
            </div>
            <span className={`${styles.duration} text-numeric`}>{item.durationLabel}</span>
            <div className={styles.rowActions}>
              {item.canMove && <>
                <button className={`${styles.moveButton} ${controls.focusRing}`} type="button" aria-label={i18n("today.item.moveEarlier", { title: item.title })} disabled={props.busy || orderedItems.findIndex((ordered) => ordered.id === item.id) === 0} onClick={() => props.onMove(item.id, -1)}>↑</button>
                <button className={`${styles.moveButton} ${controls.focusRing}`} type="button" aria-label={i18n("today.item.moveLater", { title: item.title })} disabled={props.busy || orderedItems.findIndex((ordered) => ordered.id === item.id) === orderedItems.length - 1} onClick={() => props.onMove(item.id, 1)}>↓</button>
              </>}
              {item.canRemove && <button className={`${styles.textAction} ${controls.focusRing}`} type="button" disabled={props.busy} onClick={() => props.onRemove(item.id)}>{i18n("today.item.remove")}</button>}
              {item.canHide && <button className={`${styles.textAction} ${controls.focusRing}`} type="button" disabled={props.busy} onClick={() => props.onHideSchedule(item.id)}>{i18n("today.item.hide")}</button>}
            </div>
          </article>)}
        </div>}
        <footer className={`${styles.total} text-body-strong`}><span>{i18n("today.items.total", { count: props.totalCount })}</span><span>{props.totalTimeLabel}</span></footer>
      </section>
      <aside className={styles.sideColumn}>
        <section className={`${controls.card} ${styles.sideCard}`} aria-labelledby="today-progress-title">
          <h2 className="text-card-title" id="today-progress-title">{i18n("today.progress.title")}</h2>
          <p className={styles.progressCount}><strong className="text-page-title">{props.completedCount} / {props.totalCount}</strong> <span className="text-body">{i18n("today.progress.itemsDone")}</span></p>
          <div className={styles.progressTrack} role="progressbar" aria-label={i18n("today.progress.title")} aria-valuemin={0} aria-valuemax={100} aria-valuenow={Math.min(100, progress)}>
            <span className={styles.progressFill} style={{ width: `${Math.min(100, progress)}%` }} />
          </div>
          <dl className={styles.progressTimes}><div><dt className="text-caption">{i18n("today.progress.completedTime")}</dt><dd className="text-numeric">{props.completedTimeLabel}</dd></div><div><dt className="text-caption">{i18n("today.progress.remainingTime")}</dt><dd className="text-numeric">{props.remainingTimeLabel}</dd></div></dl>
        </section>
        <section className={`${controls.card} ${styles.sideCard}`} aria-labelledby="today-next-title">
          <h2 className="text-card-title" id="today-next-title">{i18n("today.next.title")}</h2>
          {props.nextSchedule ? <>
            <p className={styles.nextTime}><strong className="text-page-title">{props.nextSchedule.startLabel}</strong><span className="text-body">– {props.nextSchedule.endLabel}</span></p>
            <p className={`${styles.nextName} text-body-strong`}>{props.nextSchedule.title}</p>
            <p className={styles.countdown}><span className="text-caption">{i18n("today.next.startsIn")}</span><strong className="text-body-strong">{props.nextSchedule.countdownLabel}</strong></p>
          </> : <p className={`${styles.emptyNext} text-body`}>{i18n("today.next.none")}</p>}
        </section>
        {props.hiddenScheduleCount > 0 && <button className={`${controls.button} ${controls.outlineButton} ${controls.focusRing} ${styles.hiddenButton} text-button-small`} type="button" disabled={props.busy} onClick={props.onShowHiddenSchedules}>{i18n("today.hiddenSchedules", { count: props.hiddenScheduleCount })}</button>}
      </aside>
    </div>
  </section>
}
