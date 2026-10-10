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
    { clientKey: "recurring", seriesId: "series-1", occurrenceDate: "2026-10-10", isRecurring: true, title: "Repeat", estimatedMinutes: "0:30", priority: "high" },
    { clientKey: "oneoff", seriesId: "once-1", occurrenceDate: "2026-10-10", title: "One off", estimatedMinutes: "", priority: "low" },
  ],
}

describe("taskFormValues2taskUpdateRequest", () => {
  it("includes only Task fields marked dirty", () => {
    const current = structuredClone(base)
    current.title = "Renamed"
    current.manualEstimate = "2:00"
    const dirtyFields = { title: true, manualEstimate: true } as TaskFormDirtyFields

    expect(taskFormValues2taskUpdateRequest(current, dirtyFields)).toEqual({ title: "Renamed", manual_estimated_minutes: 120 })
  })

  it("does not update untouched recurring occurrences", () => {
    expect(taskFormValues2actionItemSaveOperations(base, {} as TaskFormDirtyFields)).toEqual([])
  })

  it("updates only changed fields of dirty current occurrences", () => {
    const current = structuredClone(base)
    current.actionItems[0]!.title = "Edited occurrence"
    const dirtyFields = { actionItems: [{ title: true }] } as TaskFormDirtyFields

    expect(taskFormValues2actionItemSaveOperations(current, dirtyFields)).toEqual([{
      type: "update",
      key: "recurring",
      index: 0,
      item: current.actionItems[0],
      seriesId: "series-1",
      data: { scope: "current", occurrence_date: "2026-10-10", title: "Edited occurrence" },
    }])
  })

  it("creates new rows and keeps deletes as per-row operations", () => {
    const current = structuredClone(base)
    current.actionItems.push({ clientKey: "new-1", title: "New item", estimatedMinutes: "0:30", priority: "medium" })
    current.actionItems[1]!.pendingDelete = true
    const dirtyFields = { actionItems: [{}, { pendingDelete: true }] } as TaskFormDirtyFields

    expect(taskFormValues2actionItemSaveOperations(current, dirtyFields)).toEqual([
      { type: "delete", key: "oneoff", index: 1, item: current.actionItems[1] },
      { type: "create", key: "new-1", index: 2, item: current.actionItems[2], data: { title: "New item", estimated_minutes: 30, priority: "medium" } },
    ])
  })

  it("discards an unsaved tombstone without making an API request", () => {
    const current = structuredClone(base)
    current.actionItems.push({ clientKey: "new-1", pendingDelete: true, title: "Draft", estimatedMinutes: "", priority: "" })

    expect(taskFormValues2actionItemSaveOperations(current, {} as TaskFormDirtyFields)).toEqual([
      { type: "discard", key: "new-1", index: 2 },
    ])
  })
})
