import { afterEach, expect, test, vi } from "vitest"
import { cleanup, createEvent, fireEvent, render, screen } from "@testing-library/react"
import { LanguageProviderContext } from "~/features/i18n/languageContext"
import TodayBoard from "."
import type { TodayBoardProps } from "./types"

afterEach(cleanup)

function renderBoard(overrides: Partial<TodayBoardProps> = {}) {
  const props: TodayBoardProps = {
    dateLabel: "2026-10-12",
    items: [
      { id: "action-1", kind: "action_item", title: "Write outline", groupLabel: "Project · Task", status: "today.status.work", timeLabel: "—", durationLabel: "0:30", completed: false, history: false, canComplete: true, canReopen: false, canMove: true, canRemove: true, canHide: false },
      { id: "schedule-1", kind: "schedule", title: "Team review", groupLabel: "Project", status: "today.status.scheduled", timeLabel: "13:00", durationLabel: "0:30", completed: false, history: false, canComplete: false, canReopen: false, canMove: true, canRemove: false, canHide: true },
    ],
    completedCount: 0,
    totalCount: 2,
    totalTimeLabel: "1:00",
    completedTimeLabel: "0:00",
    remainingTimeLabel: "1:00",
    hiddenScheduleCount: 1,
    onComplete: vi.fn(), onReopen: vi.fn(), onMove: vi.fn(), onReorder: vi.fn(), onRemove: vi.fn(), onHideSchedule: vi.fn(),
    onShowHiddenSchedules: vi.fn(), onOpenHelp: vi.fn(), onRegenerate: vi.fn(),
    ...overrides,
  }
  render(<LanguageProviderContext.Provider value={{ language: "en", setLanguage: vi.fn() }}><TodayBoard {...props} /></LanguageProviderContext.Provider>)
  return props
}

test("renders Today execution plan without a separate top list", () => {
  renderBoard()
  expect(screen.getByRole("heading", { name: "Today's tasks" })).toBeTruthy()
  expect(screen.getByRole("heading", { name: "Today's progress" })).toBeTruthy()
  expect(screen.getByRole("heading", { name: "Next schedule" })).toBeTruthy()
  expect(screen.queryByRole("heading", { name: "Things to do" })).toBeNull()
})

test("exposes a manual regenerate action", () => {
  const props = renderBoard()
  fireEvent.click(screen.getByRole("button", { name: "Regenerate today's list" }))
  expect(props.onRegenerate).toHaveBeenCalledOnce()
})

test("keyboard move controls and dragging schedule rows change display order", () => {
  const props = renderBoard()
  fireEvent.click(screen.getByRole("button", { name: "Move Write outline later" }))
  expect(props.onMove).toHaveBeenCalledWith("action-1", 1)
  const scheduleRow = screen.getByText("Team review").closest("article")
  expect(scheduleRow).toBeTruthy()
  fireEvent.dragStart(scheduleRow!)
  expect(scheduleRow?.getAttribute("draggable")).toBe("true")
})

test("allows dropping ActionItems before or after a fixed Schedule anchor", () => {
  const props = renderBoard()
  const source = screen.getByText("Write outline").closest("article")!
  fireEvent.dragStart(source)
  const target = screen.getByText("Team review").closest("article")!
  vi.spyOn(target, "getBoundingClientRect").mockReturnValue({ top: 100, bottom: 140, left: 0, right: 100, width: 100, height: 40, x: 0, y: 100, toJSON: () => ({}) })
  const drop = createEvent.drop(target, { clientY: 110 })
  Object.defineProperty(drop, "clientY", { value: 110 })
  fireEvent(target, drop)
  expect(props.onReorder).toHaveBeenCalledWith("action-1", "schedule-1", "before")
})

test("allows dropping ActionItems around completed fixed rows", () => {
  const props = renderBoard({ items: [
    { id: "action-1", kind: "action_item", title: "Write outline", groupLabel: "Project · Task", status: "today.status.work", timeLabel: "—", durationLabel: "0:30", completed: false, history: false, canComplete: true, canReopen: false, canMove: true, canRemove: true, canHide: false },
    { id: "completed-1", kind: "schedule", title: "Finished review", groupLabel: "Project", status: "today.status.completed", timeLabel: "10:00", durationLabel: "0:30", completed: true, history: false, canComplete: false, canReopen: true, canMove: false, canRemove: false, canHide: false },
  ] })
  const source = screen.getByText("Write outline").closest("article")!
  fireEvent.dragStart(source)
  const target = screen.getByText("Finished review").closest("article")!
  vi.spyOn(target, "getBoundingClientRect").mockReturnValue({ top: 100, bottom: 140, left: 0, right: 100, width: 100, height: 40, x: 0, y: 100, toJSON: () => ({}) })
  const drop = createEvent.drop(target, { clientY: 135 })
  Object.defineProperty(drop, "clientY", { value: 135 })
  fireEvent(target, drop)
  expect(props.onReorder).toHaveBeenCalledWith("action-1", "completed-1", "after")
  expect(target.getAttribute("draggable")).toBe("false")
})

test("does not restrict work order around fixed schedule times", () => {
  const props = renderBoard()
  expect(props.items.filter((item) => !item.history).map((item) => item.id)).toEqual(["action-1", "schedule-1"])
})
