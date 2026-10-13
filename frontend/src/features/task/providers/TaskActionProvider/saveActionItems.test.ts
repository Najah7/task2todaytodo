import { afterEach, expect, test, vi } from "vitest"
import { ApiError } from "~/api/http"
import type { TaskActionItemSaveOperation } from "~/features/task/converters/taskFormValues2updateRequest"

const api = vi.hoisted(() => ({ patch: vi.fn(), create: vi.fn(), remove: vi.fn(), getTask: vi.fn() }))
vi.mock("~/api/generated/tasks", () => ({
  patchTasksTaskIdActionItemsId: api.patch,
  postTasksTaskIdActionItems: api.create,
  deleteTasksTaskIdActionItemsId: api.remove,
  getTasksId: api.getTask,
}))

import { saveTaskActionItems } from "./saveActionItems"

afterEach(() => vi.resetAllMocks())

test("saves ActionItem updates without Todo regeneration confirmation", async () => {
  api.patch
    .mockResolvedValueOnce({ id: "one", title: "First", estimated_minutes: 30 })
    .mockResolvedValueOnce({ id: "two", title: "Second", estimated_minutes: 20 })
  const operations = [
    updateOperation("one", "first"),
    updateOperation("two", "second"),
  ]

  const result = await saveTaskActionItems("task-1", operations)
  expect(result.hasSavedItems).toBe(true)
  expect(result.outcomes).toHaveLength(2)
  expect(api.patch).toHaveBeenLastCalledWith("task-1", "two", {
    title: "second",
  })
})

test("creates child ActionItems without regeneration flags", async () => {
  api.create.mockResolvedValueOnce({ id: "created", title: "First child", estimated_minutes: 30 })
  const operation: TaskActionItemSaveOperation = {
    type: "create", key: "new-child", index: 0,
    item: { title: "First child", estimatedMinutes: "0:30", priority: "", actualMinutes: "" },
    data: { title: "First child", estimated_minutes: 30 },
  }

  const result = await saveTaskActionItems("task-1", [operation])
  expect(result.outcomes).toHaveLength(1)
  expect(api.create).toHaveBeenLastCalledWith("task-1", { title: "First child", estimated_minutes: 30 })
})

test("last child deletion sends manual estimate with current Task revision atomically", async () => {
  api.remove.mockResolvedValueOnce(undefined)
  const operation: TaskActionItemSaveOperation = {
    type: "delete", key: "last-child", index: 0, manualEstimatedMinutes: 75,
    item: { id: "child-1", title: "Last child", estimatedMinutes: "0:45", priority: "", actualMinutes: "" },
  }

  await saveTaskActionItems("task-1", [operation], 9)

  expect(api.remove).toHaveBeenCalledWith("task-1", "child-1", {
    manual_estimated_minutes: 75,
  }, { headers: { "If-Match": '"9"' } })
})

test("turns stale Task revision into form conflict instead of row-level save error", async () => {
  api.remove.mockRejectedValueOnce(new ApiError(412, { error: { details: [{ code: "revision_mismatch" }] } }))
  const result = await saveTaskActionItems("task-1", [{
    type: "delete", key: "last-child", index: 0, manualEstimatedMinutes: 75,
    item: { id: "child-1", title: "Last child", estimatedMinutes: "0:45", priority: "", actualMinutes: "" },
  }], 9)
  expect(result.revisionConflict).toBe(true)
  expect(result.outcomes).toEqual([])
})

test("does not overwrite externally changed manual estimate after own child save", async () => {
  api.create.mockResolvedValueOnce({ id: "new-child", title: "Replacement", estimated_minutes: 30 })
  api.getTask.mockResolvedValueOnce({ task: { id: "task-1", revision: 9, manual_estimated_minutes: 90 } })
  const operations: TaskActionItemSaveOperation[] = [
    {
      type: "create", key: "replacement", index: 0,
      item: { title: "Replacement", estimatedMinutes: "0:30", priority: "", actualMinutes: "" },
      data: { title: "Replacement", estimated_minutes: 30 },
    },
    {
      type: "delete", key: "last-child", index: 1, manualEstimatedMinutes: 45,
      item: { id: "old-child", title: "Old child", estimatedMinutes: "0:30", priority: "", actualMinutes: "" },
    },
  ]

  const result = await saveTaskActionItems("task-1", operations, 7, 30)

  expect(result.revisionConflict).toBe(true)
  expect(result.outcomes).toHaveLength(1)
  expect(result.pendingOperations).toEqual([operations[1]])
  expect(api.remove).not.toHaveBeenCalled()
})

test("keeps old children when a replacement child create fails", async () => {
  api.create.mockRejectedValueOnce(new Error("create failed"))
  const operations: TaskActionItemSaveOperation[] = [
    {
      type: "create", key: "replacement", index: 0,
      item: { title: "Replacement", estimatedMinutes: "0:30", priority: "", actualMinutes: "" },
      data: { title: "Replacement", estimated_minutes: 30 },
    },
    {
      type: "delete", key: "old-child", index: 1,
      item: { id: "old-child", title: "Old child", estimatedMinutes: "0:30", priority: "", actualMinutes: "" },
    },
  ]

  const result = await saveTaskActionItems("task-1", operations)

  expect(result.outcomes).toHaveLength(1)
  expect(result.outcomes[0]).toMatchObject({ key: "replacement", error: "tasks.form.actionItemSaveError" })
  expect(api.remove).not.toHaveBeenCalled()
})

test("creates replacement children before deleting existing children", async () => {
  const calls: string[] = []
  api.create.mockImplementationOnce(async () => { calls.push("create"); return { id: "new-child", title: "New", estimated_minutes: 30 } })
  api.remove.mockImplementationOnce(async () => { calls.push("delete") })
  const operations: TaskActionItemSaveOperation[] = [
    {
      type: "create", key: "replacement", index: 0,
      item: { title: "Replacement", estimatedMinutes: "0:30", priority: "", actualMinutes: "" },
      data: { title: "Replacement", estimated_minutes: 30 },
    },
    {
      type: "delete", key: "old-child", index: 1,
      item: { id: "old-child", title: "Old child", estimatedMinutes: "0:30", priority: "", actualMinutes: "" },
    },
  ]

  await saveTaskActionItems("task-1", operations)

  expect(calls).toEqual(["create", "delete"])
})

function updateOperation(actionItemId: string, title: string): TaskActionItemSaveOperation {
  return {
    type: "update",
    actionItemId,
    key: actionItemId,
    index: 0,
    item: { id: actionItemId, title, estimatedMinutes: "0:30", priority: "", actualMinutes: "" },
    data: { title },
  }
}
