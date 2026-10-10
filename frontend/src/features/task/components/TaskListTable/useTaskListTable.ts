import { useContext, useState } from "react"
import { useQueries, useQueryClient } from "@tanstack/react-query"
import {
  GetTasksStatus,
  getGetTasksTaskIdActionItemsQueryKey,
  type RestActionItemResponse,
} from "~/api/generated/tasks"
import { useI18n, useLanguage } from "~/features/i18n/hooks"
import { tasks2taskListRowViews } from "~/features/task/converters/tasks2taskListRowViews"
import { taskListResponses2summaryViews } from "~/features/task/converters/taskListResponses2summaryViews"
import { getAllTaskActionItems } from "~/features/task/queries"
import { TaskAPIDataContext } from "~/features/task/providers/TaskAPIDataProvider/context"
import { TaskActionContext } from "~/features/task/providers/TaskActionProvider/context"
import { TaskNavigationContext } from "~/features/task/providers/TaskNavigationProvider/context"
import type { TaskActionItemRowView, TaskActionItemsQueryState } from "./types"

export function useTaskListTable() {
  const i18n = useI18n()
  const { language } = useLanguage()
  const { tasksQuery } = useContext(TaskAPIDataContext)!
  const { busy, retry, updateActionItemCompletion } = useContext(TaskActionContext)!
  const { edit } = useContext(TaskNavigationContext)!
  const [expandedIds, setExpandedIds] = useState<string[]>([])
  const [busyActionItemKey, setBusyActionItemKey] = useState<string>()
  const queryClient = useQueryClient()
  const visibleExpandedIds = expandedIds.filter((taskId) => tasksQuery.data?.items?.some((task) => task.id === taskId))
  const actionItemQueries = useQueries({
    queries: visibleExpandedIds.map((taskId) => ({
      queryKey: [...getGetTasksTaskIdActionItemsQueryKey(taskId, { page_size: 100 }), "all-pages"],
      queryFn: () => getAllTaskActionItems(taskId),
      retry: false,
    })),
  })
  const taskItemsById = Object.fromEntries(visibleExpandedIds.map((taskId, index) => [taskId, actionItemQueries[index]?.data ?? []])) as Record<string, RestActionItemResponse[]>
  const actionItemsQueryStates = Object.fromEntries(visibleExpandedIds.map((taskId, index) => {
    const query = actionItemQueries[index]
    const status = query?.data !== undefined ? "success" : query?.isLoading || query?.isFetching ? "loading" : query?.isError ? "error" : "success"
    return [taskId, status]
  })) as Record<string, TaskActionItemsQueryState | undefined>
  const getStatusLabel = (status: string) => i18n(taskStatusLabel(status))
  const rows = tasks2taskListRowViews((tasksQuery.data?.items ?? []).map((task) => ({
    task,
    actionItems: taskItemsById[task.id ?? ""] ?? [],
    language,
    unassignedProject: i18n("tasks.project.unassigned"),
    noDeadline: i18n("tasks.deadline.none"),
    getStatusLabel,
  })))
  const summary = taskListResponses2summaryViews([tasksQuery.data], language, i18n)[0]!
  function toggleExpanded(taskId: string) {
    setExpandedIds((current) => current.includes(taskId) ? current.filter((id) => id !== taskId) : [...current, taskId])
  }

  function toggleActionItem(item: TaskActionItemRowView) {
    if (busy || busyActionItemKey) return
    setBusyActionItemKey(item.occurrenceKey)
    void updateActionItemCompletion({
      taskId: item.taskId,
      actionItemId: item.actionItemId,
      seriesId: item.seriesId,
      occurrenceDate: item.occurrenceDate,
      completed: !item.completed,
    }).catch(() => undefined).finally(() => setBusyActionItemKey(undefined))
  }

  return {
    rows,
    summary,
    expandedIds,
    busyActionItemKey,
    actionItemsQueryStates,
    loading: tasksQuery.isLoading,
    error: Boolean(tasksQuery.error && !tasksQuery.data),
    toggleExpanded,
    toggleActionItem,
    retryActionItems: (taskId: string) => void queryClient.invalidateQueries({ queryKey: getGetTasksTaskIdActionItemsQueryKey(taskId) }),
    retry,
    edit,
  }
}

function taskStatusLabel(status: string): "tasks.status.open" | "tasks.status.inProgress" | "tasks.status.pending" | "tasks.status.waitingOnOthers" | "tasks.status.done" {
  if (status === GetTasksStatus.in_progress) return "tasks.status.inProgress"
  if (status === GetTasksStatus.pending) return "tasks.status.pending"
  if (status === GetTasksStatus.waiting_on_others) return "tasks.status.waitingOnOthers"
  if (status === GetTasksStatus.done) return "tasks.status.done"
  return "tasks.status.open"
}
