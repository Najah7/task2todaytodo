import type { RestTodoItemResponse } from "~/api/generated/todo"

export function todoItems2registeredIds(items: RestTodoItemResponse[] | undefined): Set<string> {
  const ids = new Set<string>()
  for (const item of items ?? []) {
    const sourceId = item.kind === "task" ? item.task_id : item.kind === "action_item" ? item.action_item_id : undefined
    if (!sourceId || item.history) continue
    ids.add(sourceId)
  }
  return ids
}
