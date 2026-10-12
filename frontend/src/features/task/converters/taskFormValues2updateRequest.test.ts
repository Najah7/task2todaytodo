import { describe, expect, it } from "vitest"
import {
  taskFormValues2actionItemSaveOperations,
  taskFormValues2taskUpdateRequest,
  type TaskFormDirtyFields,
} from "./taskFormValues2updateRequest"
import type { TaskFormValues } from "~/features/task/components/TaskForm/schema"

const base: TaskFormValues = {
  title: "Task",
  projectId: "project-1",
  dueDate: "",
  description: "",
  priority: "high",
  manualEstimate: "1:00",
  actionItems: [
    { clientKey: "persisted-1", id: "item-1", title: "First item", estimatedMinutes: "0:30", priority: "high" },
    { clientKey: "persisted-2", id: "item-2", title: "Second item", estimatedMinutes: "", priority: "low" },
  ],
}

describe("taskFormValues2taskUpdateRequest", () => {
  it("includes only Task fields marked dirty", () => {
    const current = structuredClone(base)
    current.title = "Renamed"
    current.manualEstimate = "2:00"
    current.actionItems = []
    const dirtyFields = { title: true, manualEstimate: true } as TaskFormDirtyFields

    expect(taskFormValues2taskUpdateRequest(current, dirtyFields)).toEqual({ title: "Renamed", manual_estimated_minutes: 120 })
  })

  it("updates Task actual time only for a completed Task", () => {
    const current = { ...structuredClone(base), actionItems: [], taskCompleted: true, actualMinutes: "0" }
    expect(taskFormValues2taskUpdateRequest(current, { actualMinutes: true } as TaskFormDirtyFields)).toEqual({ actual_minutes: 0 })
    expect(taskFormValues2taskUpdateRequest({ ...current, taskCompleted: false }, { actualMinutes: true } as TaskFormDirtyFields)).toEqual({})
  })

  it("does not update untouched ActionItems", () => {
    expect(taskFormValues2actionItemSaveOperations(base, {} as TaskFormDirtyFields)).toEqual([])
  })

  it("updates only changed fields of a dirty ActionItem by concrete ID", () => {
    const current = structuredClone(base)
    current.actionItems[0]!.title = "Edited item"
    const dirtyFields = { actionItems: [{ title: true }] } as TaskFormDirtyFields

    expect(taskFormValues2actionItemSaveOperations(current, dirtyFields)).toEqual([{
      type: "update",
      key: "persisted-1",
      index: 0,
      item: current.actionItems[0],
      actionItemId: "item-1",
      data: { title: "Edited item" },
    }])
  })

  it("creates new rows and keeps deletes as per-row operations", () => {
    const current = structuredClone(base)
    current.actionItems.push({ clientKey: "new-1", title: "New item", estimatedMinutes: "0:30", priority: "medium" })
    current.actionItems[1]!.pendingDelete = true
    const dirtyFields = { actionItems: [{}, { pendingDelete: true }] } as TaskFormDirtyFields

    expect(taskFormValues2actionItemSaveOperations(current, dirtyFields)).toEqual([
      { type: "create", key: "new-1", index: 2, item: current.actionItems[2], data: { title: "New item", estimated_minutes: 30, priority: "medium" } },
      { type: "delete", key: "persisted-2", index: 1, item: current.actionItems[1] },
    ])
    expect(taskFormValues2taskUpdateRequest(current, { manualEstimate: true } as TaskFormDirtyFields)).toEqual({})
  })

  it("discards an unsaved tombstone without making an API request", () => {
    const current = structuredClone(base)
    current.actionItems.push({ clientKey: "new-1", pendingDelete: true, title: "Draft", estimatedMinutes: "", priority: "" })

    expect(taskFormValues2actionItemSaveOperations(current, {} as TaskFormDirtyFields)).toEqual([
      { type: "discard", key: "new-1", index: 2 },
    ])
  })

  it("passes a new positive manual estimate with the last ActionItem deletion", () => {
    const current: TaskFormValues = {
      ...structuredClone(base),
      manualEstimate: "0:45",
      actionItems: [{ id: "last-item", title: "Last item", estimatedMinutes: "0:30", priority: "medium", pendingDelete: true }],
    }
    expect(taskFormValues2actionItemSaveOperations(current, { manualEstimate: true } as TaskFormDirtyFields)).toEqual([{
      type: "delete",
      key: "persisted:last-item",
      index: 0,
      item: current.actionItems[0],
      manualEstimatedMinutes: 45,
    }])
    expect(taskFormValues2taskUpdateRequest(current, { manualEstimate: true } as TaskFormDirtyFields)).toEqual({})
  })

  it("creates replacements before deleting the last old child", () => {
    const current: TaskFormValues = {
      ...structuredClone(base),
      manualEstimate: "",
      actionItems: [
        { id: "old-child", title: "Old child", estimatedMinutes: "0:30", priority: "medium", pendingDelete: true },
        { clientKey: "new-child", title: "Replacement", estimatedMinutes: "0:45", priority: "high" },
      ],
    }
    const operations = taskFormValues2actionItemSaveOperations(current, {} as TaskFormDirtyFields)
    expect(operations.map((operation) => operation.type)).toEqual(["create", "delete"])
    expect(operations[1]).not.toHaveProperty("manualEstimatedMinutes")
  })
})
