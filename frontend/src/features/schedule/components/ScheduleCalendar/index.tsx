import { useContext, useEffect, useMemo, useRef, useState, type CSSProperties, type RefObject } from "react"
import { useI18n, useLanguage } from "~/features/i18n/hooks"
import ConfirmationDialog from "~/features/shared/components/ConfirmationDialog"
import { ScheduleActionContext } from "~/features/schedule/providers/ScheduleActionProvider/context"
import { ScheduleAPIDataContext, type ScheduleEvent } from "~/features/schedule/providers/ScheduleAPIDataProvider/context"
import { ScheduleNavigationContext } from "~/features/schedule/providers/ScheduleNavigationProvider/context"
import controls from "~/styles/controls.module.css"
import { calendarDates, dateKey, eventsOnDate, formatDate, timedLayouts, timeMinutes, zonedDateKey, zonedTime } from "./calendarLayout"
import styles from "./index.module.css"

const hourHeightRem = 3
export default function ScheduleCalendar() {
  const i18n = useI18n()
  const { language } = useLanguage()
  const data = useContext(ScheduleAPIDataContext)!
  const actions = useContext(ScheduleActionContext)!
  const navigation = useContext(ScheduleNavigationContext)!
  const { state } = navigation
  const dateList = useMemo(() => calendarDates(state.date, state.view, data.timezone), [state.date, state.view, data.timezone])
  const scrollRef = useRef<HTMLDivElement>(null)
  const [pendingDelete, setPendingDelete] = useState<{ event: ScheduleEvent; scope: "current" | "following" }>()

  useEffect(() => {
    const container = scrollRef.current
    if (!container || state.view === "month") return
    const position = () => {
      const header = container.querySelector<HTMLElement>(`.${styles.dayHeader}`)
      const allDay = container.querySelector<HTMLElement>(`.${styles.allDayRow}`)
      const hour = container.querySelector<HTMLElement>(`.${styles.hourSlot}`)
      if (header && allDay && hour) container.scrollTop = 9 * hour.getBoundingClientRect().height
      if (window.matchMedia("(max-width: 42em)").matches) {
        const selected = container.querySelector<HTMLElement>(`[data-date="${state.date}"]`)
        const gutter = container.querySelector<HTMLElement>(`.${styles.timeGutter}`)
        if (selected && gutter) {
          const containerLeft = container.getBoundingClientRect().left
          const selectedLeft = selected.getBoundingClientRect().left
          container.scrollLeft += selectedLeft - containerLeft - gutter.getBoundingClientRect().width
        }
      }
    }
    position()
    window.addEventListener("resize", position)
    return () => window.removeEventListener("resize", position)
  }, [dateList, state.date, state.view])

  const rangeTitle = state.view === "month"
    ? formatDate(dateList[Math.floor(dateList.length / 2)]!, language, { month: "long", year: "numeric" })
    : state.view === "day"
      ? formatDate(dateList[0]!, language, { weekday: "long", year: "numeric", month: "long", day: "numeric" })
      : `${formatDate(dateList[0]!, language, { month: "short", day: "numeric" })} – ${formatDate(dateList[6]!, language, { year: "numeric", month: "short", day: "numeric" })}`

  const slot = (date: Date, hour: number) => {
    const dateText = dateKey(date)
    const startTime = `${String(hour).padStart(2, "0")}:00`
    const endTime = hour === 23 ? "23:59" : `${String(hour + 1).padStart(2, "0")}:00`
    navigation.create({ startDate: dateText, endDate: dateText, startTime, endTime, allDay: false })
  }

  async function confirmDelete() {
    if (!pendingDelete) return
    try {
      await actions.delete(pendingDelete.event, pendingDelete.scope)
      setPendingDelete(undefined)
    } catch {
      // The action provider shows the failure notification; keep the confirmation open for retry.
    }
  }

  return (
    <section className={`${controls.card} ${styles.calendar}`} aria-label={i18n("page.schedules.title")}>
      <header className={styles.toolbar}>
        <div className={styles.rangeControls}>
          <button className={`${controls.button} ${controls.outlineButton} ${controls.focusRing} text-button-small`} type="button" onClick={navigation.today}>{i18n("schedules.today")}</button>
          <button className={`${styles.arrowButton} ${controls.focusRing}`} type="button" aria-label={i18n("schedules.previous")} onClick={navigation.previous}>‹</button>
          <button className={`${styles.arrowButton} ${controls.focusRing}`} type="button" aria-label={i18n("schedules.next")} onClick={navigation.next}>›</button>
          <h2 className="text-card-title" aria-live="polite">{rangeTitle}</h2>
        </div>
        <div className={styles.toolbarFilters}>
          <label className="text-detail" htmlFor="schedule-project-filter">{i18n("schedules.projectFilter")}</label>
          <select id="schedule-project-filter" className={`${controls.select} ${controls.focusRing} text-body`} value={state.projectId ?? ""} onChange={(event) => navigation.setProject(event.target.value || undefined)}>
            <option value="">{i18n("schedules.allProjects")}</option>
            {data.projectOptions.map((project) => <option key={project.id} value={project.id}>{project.title}</option>)}
          </select>
          <label className={`${styles.skippedToggle} text-detail`}>
            <input type="checkbox" checked={state.showSkipped} onChange={(event) => navigation.setShowSkipped(event.target.checked)} />
            {state.showSkipped ? i18n("schedules.hideSkipped") : i18n("schedules.showSkipped")}
          </label>
          <div className={styles.viewSwitch} role="group" aria-label={i18n("schedules.view.week")}>
            {(["day", "week", "month"] as const).map((view) => <button key={view} type="button" aria-pressed={state.view === view} className={`${controls.focusRing} ${state.view === view ? styles.selectedView : ""} text-button-small`} onClick={() => navigation.setView(view)}>{i18n(`schedules.view.${view}`)}</button>)}
          </div>
        </div>
      </header>

      {state.view === "month"
        ? <MonthGrid dates={dateList} events={data.events} timezone={data.timezone} language={language} busy={actions.busy} onDay={navigation.openDay} onAdd={(date) => navigation.create({ startDate: date, endDate: date, startTime: "09:00", endTime: "10:00", allDay: false })} onEvent={navigation.edit} onDelete={(event, scope) => setPendingDelete({ event, scope })} />
        : <TimeGrid dates={dateList} events={data.events} timezone={data.timezone} language={language} scrollRef={scrollRef} busy={actions.busy} onSlot={slot} onAllDay={(day) => navigation.create({ startDate: dateKey(day), endDate: dateKey(day), startTime: "09:00", endTime: "10:00", allDay: true })} onEvent={navigation.edit} onHeaderDate={navigation.openDay} onDelete={(event, scope) => setPendingDelete({ event, scope })} />}

      {data.state === "ready" && data.events.length === 0 && <p className={`${styles.empty} text-body`}>{i18n("schedules.empty")}</p>}
      <footer className={styles.footer}>
        <span className="text-caption">{i18n("schedules.timezone", { timezone: data.timezone })}</span>
        {data.state === "error" && <span className="text-caption" role="alert">{i18n("schedules.loadError")}</span>}
      </footer>

      <ConfirmationDialog
        open={Boolean(pendingDelete)}
        title={i18n("schedules.dialog.deleteTitle")}
        description={i18n(pendingDelete?.scope === "following" ? "schedules.dialog.deleteFollowingDescription" : "schedules.dialog.deleteDescription")}
        confirmLabel={i18n("schedules.dialog.confirmDelete")}
        cancelLabel={i18n("schedules.dialog.cancel")}
        destructive
        busy={actions.busy}
        onConfirm={() => void confirmDelete()}
        onCancel={() => setPendingDelete(undefined)}
      />
    </section>
  )
}

function ScheduleEventOperations({ event, busy, onEdit, onDelete }: { event: ScheduleEvent; busy: boolean; onEdit: () => void; onDelete: (scope: "current" | "following") => void }) {
  const i18n = useI18n()
  const actions = useContext(ScheduleActionContext)!
  const menuRef = useRef<HTMLDetailsElement>(null)
  const canEdit = event.permissions.canUpdate || event.permissions.canAssign
  const run = (action: () => void) => {
    if (menuRef.current) menuRef.current.open = false
    action()
  }
  const runAsync = (action: () => Promise<void>) => run(() => { void action().catch(() => undefined) })
  return (
    <details className={styles.eventOperations} ref={menuRef}>
      <summary className={`${controls.focusRing} text-detail`} aria-label={i18n("schedules.editTitle", { title: event.title })}>⋯</summary>
      <div className={styles.eventMenu}>
        {canEdit && <button type="button" disabled={busy} onClick={() => run(onEdit)}>{i18n("schedules.edit")}</button>}
        {event.permissions.canComplete && event.state === "scheduled" && <button type="button" disabled={busy} onClick={() => runAsync(() => actions.complete(event))}>{i18n("schedules.complete")}</button>}
        {event.permissions.canComplete && event.state === "completed" && <button type="button" disabled={busy} onClick={() => runAsync(() => actions.reopen(event))}>{i18n("schedules.reopen")}</button>}
        {event.permissions.canSkip && event.state === "scheduled" && <button type="button" disabled={busy} onClick={() => runAsync(() => actions.skip(event))}>{i18n("schedules.skip")}</button>}
        {event.permissions.canRestore && event.state === "skipped" && <button type="button" disabled={busy} onClick={() => runAsync(() => actions.restore(event))}>{i18n("schedules.restore")}</button>}
        {event.permissions.canDelete && <button type="button" disabled={busy} onClick={() => run(() => onDelete("current"))}>{i18n("schedules.deleteCurrent")}</button>}
        {event.permissions.canDeleteFollowing && <button type="button" disabled={busy} onClick={() => run(() => onDelete("following"))}>{i18n("schedules.deleteFollowing")}</button>}
      </div>
    </details>
  )
}

function MonthGrid({ dates, events, timezone, language, busy, onDay, onAdd, onEvent, onDelete }: { dates: Date[]; events: ScheduleEvent[]; timezone: string; language: string; busy: boolean; onDay: (date: string) => void; onAdd: (date: string) => void; onEvent: (ruleId: string, occurrenceDate: string) => void; onDelete: (event: ScheduleEvent, scope: "current" | "following") => void }) {
  const i18n = useI18n()
  const weekdays = dates.slice(0, 7)
  const currentMonth = dates[Math.floor(dates.length / 2)]?.getUTCMonth()
  const today = zonedDateKey(new Date(), timezone)
  return (
    <div className={styles.monthGrid}>
      <div className={styles.weekdayHeader}>{weekdays.map((day) => <div key={day.getUTCDay()} className="text-detail">{formatDate(day, language, { weekday: "short" })}</div>)}</div>
      <div className={styles.monthDays}>
        {dates.map((day) => {
          const key = dateKey(day)
          const dayEvents = eventsOnDate(events, key, timezone)
          return <section key={key} className={`${styles.monthDay} ${day.getUTCDay() === 0 || day.getUTCDay() === 6 ? styles.weekend : ""} ${day.getUTCMonth() !== currentMonth ? styles.outsideMonth : ""}`} aria-label={formatDate(day, language, { dateStyle: "full" })}>
            <button className={`${styles.dayNumber} ${key === today ? styles.today : ""} ${controls.focusRing} text-detail`} type="button" aria-current={key === today ? "date" : undefined} onClick={() => onDay(key)}>{day.getUTCDate()}</button>
            <div className={styles.monthEvents}>
              {dayEvents.slice(0, 3).map((event) => <div className={styles.monthEventRow} key={`${event.ruleId}:${event.occurrenceDate}`}>
                <button className={`${styles.monthEvent} ${event.state === "completed" ? styles.completedEvent : ""} ${event.state === "skipped" ? styles.skippedEvent : ""} ${controls.focusRing} text-caption`} type="button" onClick={() => onEvent(event.ruleId, event.occurrenceDate)} title={event.title}>{event.allDay ? `${i18n("schedules.allDay")} · ` : `${zonedTime(event.start, timezone)} · `}{event.title}</button>
                <ScheduleEventOperations event={event} busy={busy} onEdit={() => onEvent(event.ruleId, event.occurrenceDate)} onDelete={(scope) => onDelete(event, scope)} />
              </div>)}
              {dayEvents.length > 3 && <button className={`${styles.moreEvents} ${controls.focusRing} text-caption`} type="button" onClick={() => onDay(key)}>+{dayEvents.length - 3} {i18n("schedules.more")}</button>}
              <button className={`${styles.monthAdd} ${controls.focusRing} text-caption`} type="button" onClick={() => onAdd(key)} aria-label={`${i18n("schedules.addOnDate", { date: formatDate(day, language, { dateStyle: "medium" })})}`}>+</button>
            </div>
          </section>
        })}
      </div>
    </div>
  )
}

function TimeGrid({ dates, events, timezone, language, scrollRef, busy, onSlot, onAllDay, onEvent, onHeaderDate, onDelete }: { dates: Date[]; events: ScheduleEvent[]; timezone: string; language: string; scrollRef: RefObject<HTMLDivElement | null>; busy: boolean; onSlot: (date: Date, hour: number) => void; onAllDay: (date: Date) => void; onEvent: (ruleId: string, occurrenceDate: string) => void; onHeaderDate: (date: string) => void; onDelete: (event: ScheduleEvent, scope: "current" | "following") => void }) {
  const i18n = useI18n()
  const today = zonedDateKey(new Date(), timezone)
  const nowMinutes = timeMinutes(zonedTime(new Date(), timezone))
  const allDayRows = Math.max(0, ...dates.map((day) => eventsOnDate(events, dateKey(day), timezone).filter((event) => event.allDay).length))
  const allDayHeight = `${Math.max(2.5, allDayRows * 1.55 + 0.3)}rem`
  return (
    <div className={styles.timeGridScroll} ref={scrollRef}>
      <div className={styles.timeGrid} style={{ "--day-count": dates.length, "--all-day-height": allDayHeight } as CSSProperties}>
        <div className={styles.timeGutter}>
          <div className={styles.gutterHeader} />
          <div className={styles.gutterAllDay}>{i18n("schedules.allDay")}</div>
          {Array.from({ length: 24 }, (_, hour) => <div className={styles.hourLabel} key={hour}>{String(hour).padStart(2, "0")}:00</div>)}
        </div>
        {dates.map((day) => {
          const key = dateKey(day)
          const isToday = key === today
          const allDayEvents = eventsOnDate(events, key, timezone).filter((event) => event.allDay)
          const layouts = timedLayouts(events, key, timezone)
          return <section className={`${styles.dayColumn} ${day.getUTCDay() === 0 || day.getUTCDay() === 6 ? styles.weekend : ""}`} key={key} data-date={key} aria-label={formatDate(day, language, { dateStyle: "full" })}>
            <header className={styles.dayHeader}>
              <span className="text-detail">{formatDate(day, language, { weekday: "short" })}</span>
              <button className={`${styles.dayNumber} ${isToday ? styles.today : ""} ${controls.focusRing} text-body-strong`} type="button" aria-current={isToday ? "date" : undefined} onClick={() => onHeaderDate(key)}>{day.getUTCDate()}</button>
            </header>
            <div className={styles.allDayRow}>
              {allDayEvents.map((event) => <div className={styles.allDayEventRow} key={`${event.ruleId}:${event.occurrenceDate}`}>
                <button className={`${styles.allDayEvent} ${event.state === "completed" ? styles.completedEvent : ""} ${event.state === "skipped" ? styles.skippedEvent : ""} ${controls.focusRing} text-caption`} type="button" title={event.title} onClick={() => onEvent(event.ruleId, event.occurrenceDate)}>{event.title}</button>
                <ScheduleEventOperations event={event} busy={busy} onEdit={() => onEvent(event.ruleId, event.occurrenceDate)} onDelete={(scope) => onDelete(event, scope)} />
              </div>)}
              <button className={`${styles.addAllDay} ${controls.focusRing} text-caption`} type="button" aria-label={`${formatDate(day, language, { dateStyle: "medium" })}: ${i18n("schedules.allDay")}`} onClick={() => onAllDay(day)} />
            </div>
            <div className={styles.dayTimeline}>
              {Array.from({ length: 24 }, (_, hour) => <button key={hour} className={`${styles.hourSlot} ${controls.focusRing}`} type="button" aria-label={`${formatDate(day, language, { dateStyle: "medium" })} ${String(hour).padStart(2, "0")}:00`} onClick={() => onSlot(day, hour)} />)}
              {key === today && <div className={styles.nowIndicator} style={{ top: `${nowMinutes / 60 * hourHeightRem}rem` }} aria-label={`${i18n("schedules.now")} ${zonedTime(new Date(), timezone)}`} />}
              {layouts.map(({ event, start, end, lane, lanes }) => <div
                key={`${event.ruleId}:${event.occurrenceDate}`}
                className={styles.timedEventPosition}
                style={{ top: `${start / 60 * hourHeightRem}rem`, height: `max(1.5rem, ${Math.max(20, end - start) / 60 * hourHeightRem}rem)`, left: `calc(${lane} * 100% / ${lanes} + 0.125rem)`, width: `calc(100% / ${lanes} - 0.25rem)` }}
              >
                <button className={`${styles.timedEvent} ${event.state === "completed" ? styles.completedEvent : ""} ${event.state === "skipped" ? styles.skippedEvent : ""} ${controls.focusRing}`} type="button" title={event.title} onClick={() => onEvent(event.ruleId, event.occurrenceDate)}><strong className="text-caption">{event.title}</strong><span className="text-caption">{zonedTime(event.start, timezone)}–{zonedTime(event.end, timezone)}</span></button>
                <ScheduleEventOperations event={event} busy={busy} onEdit={() => onEvent(event.ruleId, event.occurrenceDate)} onDelete={(scope) => onDelete(event, scope)} />
              </div>)}
            </div>
          </section>
        })}
      </div>
    </div>
  )
}
