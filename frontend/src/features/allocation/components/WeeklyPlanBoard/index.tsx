import { useEffect, useRef, useState, type PointerEvent, type CSSProperties } from "react"
import { useLanguage, useI18n } from "~/features/i18n/hooks"
import { Link } from "react-router"
import { mondayOf, shiftWeek, weekDates, snapMinuteToQuarter } from "~/features/allocation/lib/week"
import { pausedRuleReasonMessageKey } from "~/features/allocation/lib/pausedRuleReason"
import { durationLabel } from "./format"
import type { WeeklyPlanBoardProps } from "./types"
import { useModalFocusTrap } from "~/features/shared/hooks/useModalFocusTrap"
import controls from "~/styles/controls.module.css"
import styles from "./index.module.css"

export default function WeeklyPlanBoard(props: WeeklyPlanBoardProps) {
  const i18n = useI18n()
  const { language } = useLanguage()
  const { weekStart, timezone, projects, days, allocations, allocatedMinutes, targetMinutes, differenceMinutes, freeMinutes, canEdit, busy, onChangeWeek, onEditPlan, onCreateAllocation, onEditAllocation, onEditActiveTime } = props
  const dates = weekDates(weekStart)
  const today = mondayOf(new Intl.DateTimeFormat("en-CA", { timeZone: timezone, year: "numeric", month: "2-digit", day: "2-digit" }).format(new Date()))
  const locale = language === "ja" ? "ja-JP" : "en-US"
  const range = new Intl.DateTimeFormat(locale, { month: "short", day: "numeric", timeZone: "UTC" })
  const dateLabel = range.format(new Date(dates[0] + "T00:00:00Z")) + " – " + range.format(new Date(dates[6] + "T00:00:00Z"))
  const [selectedProjectId, setSelectedProjectId] = useState<string>()
  const [showTodoHelp, setShowTodoHelp] = useState(false)
  const todoHelpModal = useModalFocusTrap<HTMLElement>(showTodoHelp, () => setShowTodoHelp(false))
  const [drag, setDrag] = useState<{ date: string; start: number; end: number }>()
  const calendarScroll = useRef<HTMLDivElement>(null)
  const dragStart = useRef<{ date: string; minute: number } | undefined>(undefined)
  const suppressClick = useRef(false)
  const activeProjectId = selectedProjectId ?? projects[0]?.id
  useEffect(() => {
    const scrollport = calendarScroll.current
    if (!scrollport) return
    const rootFontSize = Number.parseFloat(getComputedStyle(document.documentElement).fontSize) || 16
    scrollport.scrollTop = 7 * 3 * rootFontSize
  }, [])
  const color = (index: number) => {
    const colorIndex = Math.abs(index) % 6 + 1
    return { "--allocation-color": "var(--color-project-" + colorIndex + ")", "--allocation-soft": "var(--color-project-" + colorIndex + "-soft)" } as CSSProperties
  }
  function minuteFromPointer(event: PointerEvent<HTMLDivElement>) {
    const bounds = event.currentTarget.getBoundingClientRect()
    return snapMinuteToQuarter((event.clientY - bounds.top) / bounds.height * 1440)
  }
  function startDrag(event: PointerEvent<HTMLDivElement>, date: string) {
    if (!canEdit || busy || !activeProjectId || event.button !== 0 || (event.target as HTMLElement).closest("." + styles.allocationBlock)) return
    event.preventDefault()
    const minute = Math.min(1425, minuteFromPointer(event))
    dragStart.current = { date, minute }
    suppressClick.current = true
    setDrag({ date, start: minute, end: minute })
    event.currentTarget.setPointerCapture(event.pointerId)
  }
  function moveDrag(event: PointerEvent<HTMLDivElement>, date: string) {
    if (!dragStart.current || dragStart.current.date !== date) return
    const minute = minuteFromPointer(event)
    setDrag({ date, start: Math.min(dragStart.current.minute, minute), end: Math.max(dragStart.current.minute, minute) })
  }
  function finishDrag(event: PointerEvent<HTMLDivElement>, date: string) {
    if (!dragStart.current || dragStart.current.date !== date) return
    if (!activeProjectId) { dragStart.current = undefined; setDrag(undefined); return }
    const start = dragStart.current.minute
    const end = minuteFromPointer(event)
    const first = Math.min(start, end)
    const last = Math.max(start, end)
    dragStart.current = undefined
    setDrag(undefined)
    onCreateAllocation(activeProjectId, date, first, last > first ? last : Math.min(first + 60, 1440))
  }

  return <section className={styles.layout}>
    <aside className={styles.projectPanel} aria-label={i18n("weeklyPlan.projects.label")}>
      <header className={styles.projectHeader}>
        <h2 className="text-section-title">{i18n("weeklyPlan.projects.label")}</h2>
        {canEdit && <button className={styles.textButton} type="button" onClick={onEditPlan}>{i18n("weeklyPlan.editPlan")}</button>}
      </header>
      {projects.length === 0 ? <p className="text-body">{i18n("weeklyPlan.projects.empty")}</p> : <ul className={styles.projectList}>
        {projects.map((project) => <li key={project.id} className={styles.projectListItem}>
          <button type="button" aria-pressed={activeProjectId === project.id} className={styles.projectCard} style={color(project.colorIndex)} onClick={() => setSelectedProjectId(project.id)}>
          <div className={styles.projectName}><span className={styles.projectDot} /><strong className="text-body-strong">{project.title}</strong></div>
          <div className="text-detail">{durationLabel(project.allocatedMinutes)} / {durationLabel(project.targetMinutes)}</div>
          <div className={styles.meter}><span style={{ width: (project.targetMinutes > 0 ? Math.min(100, project.allocatedMinutes / project.targetMinutes * 100) : 0) + "%" }} /></div>
          <p className="text-caption">{project.differenceMinutes < 0 ? i18n("weeklyPlan.projectRemaining", { duration: durationLabel(project.differenceMinutes) }) : project.differenceMinutes > 0 ? i18n("weeklyPlan.projectOver", { duration: durationLabel(project.differenceMinutes) }) : i18n("weeklyPlan.targetMet")}</p>
          </button>
        </li>)}
      </ul>}
      {props.pausedRules.length > 0 && <ul className={styles.pausedRules}>
        {props.pausedRules.map((rule) => <li key={rule.ruleId}>
          <p className="text-caption">{i18n("weeklyPlan.pausedRule", { project: rule.projectTitle, reason: i18n(pausedRuleReasonMessageKey(rule.reason)) })}</p>
          <Link to={"/weekly-plan/edit?week_start=" + encodeURIComponent(weekStart) + "#rule-" + encodeURIComponent(rule.ruleId)} className="text-caption">{i18n("weeklyPlan.pausedRule.edit")}</Link>
        </li>)}
      </ul>}
      <dl className={styles.totals}>
        <div><dt>{i18n("weeklyPlan.totalAssignedTarget")}</dt><dd>{durationLabel(allocatedMinutes)} / {durationLabel(targetMinutes)}</dd></div>
        <div><dt>{i18n("weeklyPlan.difference")}</dt><dd>{differenceMinutes > 0 ? "+" : differenceMinutes < 0 ? "−" : ""}{durationLabel(differenceMinutes)}</dd></div>
        <div><dt>{i18n("weeklyPlan.freeTime")}</dt><dd>{durationLabel(freeMinutes)}</dd></div>
      </dl>
      <div className={styles.todoHelp}><p className="text-caption">{i18n("weeklyPlan.todoHelp")}</p><button type="button" onClick={() => setShowTodoHelp(true)}>{i18n("weeklyPlan.todoLink")}</button></div>
      {showTodoHelp && <div className={styles.helpOverlay} role="presentation" onMouseDown={(event) => { if (event.target === event.currentTarget) setShowTodoHelp(false) }}><section ref={todoHelpModal.ref} onKeyDown={todoHelpModal.onKeyDown} role="dialog" aria-modal="true" aria-labelledby="todo-help-title" className={styles.helpDialog}><h2 id="todo-help-title" className="text-section-title">{i18n("weeklyPlan.todoLink")}</h2><p className="text-body">{i18n("weeklyPlan.todoHelp")}</p><button className={controls.button} type="button" onClick={() => setShowTodoHelp(false)}>{i18n("common.close")}</button></section></div>}
    </aside>
    <section className={styles.calendarPanel} aria-label={i18n("weeklyPlan.calendar.label")}>
      <header className={styles.calendarToolbar}>
        <button type="button" className={controls.button} onClick={() => onChangeWeek(today)}>{i18n("weeklyPlan.thisWeek")}</button>
        <button type="button" className={controls.button} aria-label={i18n("weeklyPlan.previousWeek")} onClick={() => onChangeWeek(shiftWeek(weekStart, -1))}>‹</button>
        <button type="button" className={controls.button} aria-label={i18n("weeklyPlan.nextWeek")} onClick={() => onChangeWeek(shiftWeek(weekStart, 1))}>›</button>
        <h2 className="text-section-title">{dateLabel}</h2>
      </header>
      <div className={styles.calendarScroll} ref={calendarScroll} data-testid="weekly-plan-calendar-scroll"><div className={styles.calendarGrid}>
        <div className={styles.timeHeader}>{timezone}</div>
        {dates.map((date) => {
          const dateValue = new Date(date + "T00:00:00Z")
          const label = new Intl.DateTimeFormat(locale, { weekday: "short", timeZone: "UTC" }).format(dateValue)
          const day = days.find((item) => item.date === date)
          const dayAllocations = allocations.filter((item) => item.date === date)
          const dayActiveTime = props.activeIntervals.filter((item) => item.date === date)
          return <section key={date} data-date={date} className={styles.dayColumn} aria-label={new Intl.DateTimeFormat(locale, { dateStyle: "full", timeZone: "UTC" }).format(dateValue)}>
            <header className={styles.dayHeader}><span>{label}</span><strong>{dateValue.getUTCDate()}</strong><span>{durationLabel(day?.allocatedMinutes ?? 0)}</span><button type="button" disabled={!canEdit || busy} className={styles.activeTimeSettings} aria-label={i18n("weeklyPlan.activeTime.editDay", { start: day?.activeStartAt || "07:00", end: day?.activeEndAt || "23:00" })} onClick={() => onEditActiveTime(date)}><span>{i18n("weeklyPlan.activeTime.range", { start: day?.activeStartAt || "07:00", end: day?.activeEndAt || "23:00" })}</span><span>{i18n("weeklyPlan.activeTime.available")}{day?.hasActiveException ? " · " + i18n("weeklyPlan.activeTime.overrideMark") : ""}</span></button></header>
            <div className={styles.dayTimeline} onPointerDown={(event) => startDrag(event, date)} onPointerMove={(event) => moveDrag(event, date)} onPointerUp={(event) => finishDrag(event, date)}>
              {drag?.date === date && drag.end > drag.start && <div aria-hidden="true" className={styles.dragSelection} style={{ top: drag.start / 60 * 3 + "rem", height: (drag.end - drag.start) / 60 * 3 + "rem" }} />}
              {dayActiveTime.map((active, index) => <div key={index} aria-hidden="true" className={styles.activeTimeBlock} style={{ top: (active.startMinute / 60 * 3) + "rem", height: ((active.endMinute - active.startMinute) / 60 * 3) + "rem" }} />)}
              {Array.from({ length: 24 }, (_, hour) => <button key={hour} type="button" disabled={!canEdit || busy || !activeProjectId} className={styles.hourSlot} aria-label={date + " " + String(hour).padStart(2, "0") + ":00 " + i18n("weeklyPlan.createAllocation")} onClick={() => { if (suppressClick.current) { suppressClick.current = false; return } if (activeProjectId) onCreateAllocation(activeProjectId, date, hour * 60, (hour + 1) * 60) }} />)}
              {dayAllocations.map((allocation) => <button key={allocation.id} type="button" disabled={!allocation.canUpdate || busy} className={styles.allocationBlock} style={{ ...color(allocation.colorIndex), top: (allocation.startMinute / 60 * 3) + "rem", height: "max(1.5rem, " + ((allocation.endMinute - allocation.startMinute) / 60 * 3) + "rem)" }} onClick={() => onEditAllocation(allocation)} aria-label={allocation.projectTitle + ", " + allocation.startLabel + "–" + allocation.endLabel}>
                <strong>{allocation.projectTitle}</strong><span>{allocation.startLabel}–{allocation.endLabel}</span>
              </button>)}
            </div>
          </section>
        })}
        <div className={styles.timeGutter} aria-hidden="true">{Array.from({ length: 24 }, (_, hour) => <span key={hour} style={{ top: hour * 3 + "rem" }}>{String(hour).padStart(2, "0")}:00</span>)}</div>
      </div></div>
      <footer className={styles.calendarLegend}><span className={styles.legendAllocation}>{i18n("weeklyPlan.legendAllocation")}</span><span className={styles.legendActiveTime}>{i18n("weeklyPlan.legendActiveTime")}</span></footer>
    </section>
  </section>
}
