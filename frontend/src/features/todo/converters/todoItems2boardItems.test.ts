import { expect, test } from "vitest"
import { civilDateLabel, todoItems2boardItems } from "./todoItems2boardItems"

test("formats time in the Todo timezone and keeps the civil date independent of it", () => {
  const [item] = todoItems2boardItems([{ id: "s", kind: "schedule", scheduled_start: "2026-10-12T13:00:00Z", scheduled_end: "2026-10-12T13:45:00Z", duration_minutes: 20, planned_minutes: null }], "America/Los_Angeles", "en-US")
  expect(item?.timeLabel).toBe("06:00")
  expect(item?.durationLabel).toBe("0:20")
  expect(civilDateLabel("2026-10-12", "en-US")).toContain("October 12, 2026")
})

test("shows history without a time and preserves backend started explanation capability", () => {
  const [history, started] = todoItems2boardItems([
    { id: "h", kind: "action_item", history: true, scheduled_start: "2026-10-12T09:00:00Z", planned_minutes: 15 },
    { id: "a", kind: "action_item", started: true, can_remove: false },
  ], "UTC", "en-US")
  expect(history?.timeLabel).toBe("—")
  expect(started?.removalBlockedMessage).toBe("today.item.startedRemoveWarning")
})

test("keeps a direct childless Task as its own Today work kind", () => {
  const [item] = todoItems2boardItems([
    { id: "task-row", kind: "task", task_id: "task-1", title: "Direct work", planned_minutes: 45, can_complete: true, completes_source: true },
  ], "UTC", "en-US")
  expect(item).toMatchObject({ kind: "task", title: "Direct work", durationLabel: "0:45", canComplete: true })
})
