import { describe, expect, it } from "vitest"
import { todoItems2registeredIds } from "./todoItems2registeredIds"

describe("todoItems2registeredIds", () => {
  it("tracks selected Task and ActionItem rows without using timestamps", () => {
    const ids = todoItems2registeredIds([
      { kind: "task", task_id: "task-1", position: 0 },
      { kind: "task", task_id: "task-1", history: true, position: 1 },
      { kind: "action_item", action_item_id: "action-1", position: 2 },
      { kind: "action_item", action_item_id: "action-1", position: 3 },
    ])

    expect(ids).toEqual(new Set(["task-1", "action-1"]))
  })
  it("keeps selected work without time data and ignores history", () => {
    const ids = todoItems2registeredIds([
      { kind: "action_item", action_item_id: "action-1", position: 0 },
      { kind: "action_item", action_item_id: "action-1", history: true, position: 1 },
      { kind: "action_item", action_item_id: "action-2", position: 2 },
    ])

    expect(ids).toEqual(new Set(["action-1", "action-2"]))
  })
})
