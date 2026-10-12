import { describe, expect, it } from "vitest"
import { todoItems2registrationTimeLabels } from "./todoItems2registrationTimeLabels"

describe("todoItems2registrationTimeLabels", () => {
  it("uses the first scheduled work row for both Task and ActionItem and ignores history", () => {
    const labels = todoItems2registrationTimeLabels([
      { kind: "task", task_id: "task-1", scheduled_start: "2026-10-12T08:00:00Z" },
      { kind: "task", task_id: "task-1", history: true, scheduled_start: null },
      { kind: "action_item", action_item_id: "action-1", scheduled_start: "2026-10-12T08:30:00Z" },
      { kind: "action_item", action_item_id: "action-1", scheduled_start: "2026-10-12T09:00:00Z" },
    ], "UTC", "en-US")

    expect(labels.get("task-1")).toBe("08:00")
    expect(labels.get("action-1")).toBe("08:30")
  })
  it("uses the first current ActionItem time and ignores history and later split frames", () => {
    const labels = todoItems2registrationTimeLabels([
      { kind: "action_item", action_item_id: "action-1", scheduled_start: "2026-10-12T00:30:00Z" },
      { kind: "action_item", action_item_id: "action-1", history: true, scheduled_start: null },
      { kind: "action_item", action_item_id: "action-1", scheduled_start: "2026-10-12T03:00:00Z" },
      { kind: "action_item", action_item_id: "action-2", scheduled_start: null },
    ], "Asia/Tokyo", "en-US")

    expect(labels.get("action-1")).toBe("09:30")
    expect(labels.has("action-2")).toBe(false)
  })
})
