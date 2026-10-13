import { useCallback, useState, type ReactNode } from "react"
import { useQueryClient } from "@tanstack/react-query"
import {
  getGetTodayTodoQueryKey,
  addTodayTodoActionItem,
  completeTodayTodoItem,
  removeTodayTodoItem,
  reopenTodayTodoItem,
  hideTodayTodoSchedule,
  restoreTodayTodoSchedule,
  reorderTodayTodo,
  regenerateTodayTodo,
  type RestTodoTodayResponse,
} from "~/api/generated/todo"
import { getGetProjectsIdTasksQueryKey, getGetTasksQueryKey, getGetTasksTaskIdActionItemsQueryKey } from "~/api/generated/tasks"
import { getGetProjectsIdQueryKey, getGetProjectsQueryKey } from "~/api/generated/projects"
import { getListInboxQueryKey } from "~/api/generated/inbox"
import { getGetProjectsIdSchedulesQueryKey, getGetSchedulesQueryKey } from "~/api/generated/schedules"
import { getGetTodoCompletionMetricsQueryKey } from "~/api/generated/metrics"
import { notify } from "~/features/shared/notification"
import { useI18n } from "~/features/i18n/hooks"
import type { MessageKey } from "~/features/i18n/messages/types"
import { TodayActionContext } from "./context"
import { todoRevisionHeaders } from "~/features/todo/actions/revisionHeaders"

type Props = { children: ReactNode }

export function TodayActionProvider({ children }: Props) {
  const queryClient = useQueryClient()
  const i18n = useI18n()
  const [busyCount, setBusyCount] = useState(0)
  const busy = busyCount > 0

  const mutate = useCallback(async (operation: (headers: HeadersInit) => Promise<RestTodoTodayResponse>, successMessage?: MessageKey): Promise<boolean> => {
    const key = getGetTodayTodoQueryKey()
    const current = queryClient.getQueryData<RestTodoTodayResponse>(key)
    if (current?.revision === undefined) {
      notify.error(i18n("today.error"))
      return false
    }
    setBusyCount((count) => count + 1)
    try {
      const snapshot = await operation(todoRevisionHeaders(current.revision))
      queryClient.setQueryData(key, snapshot)
      if (successMessage) notify.success(i18n(successMessage))
      return true
    } catch {
      await queryClient.refetchQueries({ queryKey: key, type: "active" })
      notify.error(i18n("today.error"))
      return false
    } finally {
      setBusyCount((count) => Math.max(0, count - 1))
    }
  }, [i18n, queryClient])

  const invalidateSourceProgress = useCallback(async (itemId: string) => {
    const item = queryClient.getQueryData<RestTodoTodayResponse>(getGetTodayTodoQueryKey())?.items?.find((row) => row.id === itemId)
    if (item?.kind === "schedule") {
      const invalidations = [
        queryClient.invalidateQueries({ queryKey: ["schedules"] }),
        queryClient.invalidateQueries({ queryKey: getListInboxQueryKey() }),
        queryClient.invalidateQueries({ queryKey: getGetSchedulesQueryKey() }),
        queryClient.invalidateQueries({ queryKey: getGetProjectsQueryKey() }),
      ]
      if (item.project_id) invalidations.push(
        queryClient.invalidateQueries({ queryKey: getGetProjectsIdQueryKey(item.project_id) }),
        queryClient.invalidateQueries({ queryKey: getGetProjectsIdSchedulesQueryKey(item.project_id) }),
      )
      await Promise.allSettled(invalidations)
      return
    }
    if (item?.kind !== "action_item" && item?.kind !== "task") return
    const invalidations = [
      queryClient.invalidateQueries({ queryKey: getGetTasksQueryKey() }),
      queryClient.invalidateQueries({ queryKey: getListInboxQueryKey() }),
      queryClient.invalidateQueries({ queryKey: getGetProjectsQueryKey() }),
    ]
    if (item?.kind === "action_item" && item.task_id) invalidations.push(queryClient.invalidateQueries({ queryKey: getGetTasksTaskIdActionItemsQueryKey(item.task_id) }))
    if (item.project_id) invalidations.push(
      queryClient.invalidateQueries({ queryKey: getGetProjectsIdQueryKey(item.project_id) }),
      queryClient.invalidateQueries({ queryKey: getGetProjectsIdTasksQueryKey(item.project_id) }),
    )
    await Promise.allSettled(invalidations)
  }, [queryClient])

  async function complete(itemId: string, actualMinutes?: number) {
    const saved = await mutate((headers) => completeTodayTodoItem(itemId, actualMinutes === undefined ? {} : { actual_minutes: actualMinutes }, { headers }), "today.toast.completed")
    if (saved) await invalidateSourceProgress(itemId)
    return saved
  }

  async function reopen(itemId: string) {
    const saved = await mutate((headers) => reopenTodayTodoItem(itemId, { headers }), "today.toast.reopened")
    if (saved) await invalidateSourceProgress(itemId)
    return saved
  }

  async function regenerate() {
    const saved = await mutate((headers) => regenerateTodayTodo({ headers }), "today.toast.regenerated")
    if (saved) await queryClient.invalidateQueries({ queryKey: getGetTodoCompletionMetricsQueryKey() })
    return saved
  }

  const value = {
    busy,
    complete,
    reopen,
    remove: (itemId: string) => mutate((headers) => removeTodayTodoItem(itemId, { headers }), "today.toast.removed"),
    hide: (itemId: string) => mutate((headers) => hideTodayTodoSchedule(itemId, { headers }), "today.toast.hidden"),
    restore: (itemId: string) => mutate((headers) => restoreTodayTodoSchedule(itemId, { headers }), "today.toast.restored"),
    addActionItem: (actionItemId: string) => mutate((headers) => addTodayTodoActionItem(actionItemId, { headers }), "today.toast.added"),
    reorder: (itemIds: string[]) => mutate((headers) => reorderTodayTodo({ item_ids: itemIds }, { headers })),
    regenerate,
  }

  return <TodayActionContext.Provider value={value}>{children}</TodayActionContext.Provider>
}
