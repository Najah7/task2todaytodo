import { GetTasksStatus, type RestTaskListResponse } from "~/api/generated/tasks"
import type { MessageKey } from "~/features/i18n/messages/types"

type TaskListSummaryView = {
  totalLabel: string
  completedActionItemsLabel: string
  totalActionItemsLabel: string
  completedActionItems: number
  totalActionItems: number
  estimateTotalLabel: string
}

type TaskStatusTabView = { value: string; label: MessageKey; count: number; selected: boolean }

const statuses = [GetTasksStatus.open, GetTasksStatus.in_progress, GetTasksStatus.pending, GetTasksStatus.waiting_on_others, GetTasksStatus.done] as const
const statusLabels: Record<typeof statuses[number], MessageKey> = {
  [GetTasksStatus.open]: "tasks.status.open",
  [GetTasksStatus.in_progress]: "tasks.status.inProgress",
  [GetTasksStatus.pending]: "tasks.status.pending",
  [GetTasksStatus.waiting_on_others]: "tasks.status.waitingOnOthers",
  [GetTasksStatus.done]: "tasks.status.done",
}

export function taskListResponses2summaryViews(responses: (RestTaskListResponse | undefined)[], language: string, translate: (key: MessageKey, values?: Record<string, string | number>) => string): TaskListSummaryView[] {
  return responses.map((response) => {
    const formatNumber = new Intl.NumberFormat(language === "ja" ? "ja-JP" : "en-US")
    return {
      totalLabel: translate("tasks.summary.taskCount", { count: formatNumber.format(response?.total_count ?? 0) }),
      completedActionItemsLabel: formatNumber.format(response?.action_item_completed_count ?? 0),
      totalActionItemsLabel: formatNumber.format(response?.action_item_total_count ?? 0),
      completedActionItems: response?.action_item_completed_count ?? 0,
      totalActionItems: response?.action_item_total_count ?? 0,
      estimateTotalLabel: formatMinutes(response?.estimated_minutes_total),
    }
  })
}

export function taskListResponses2statusTabs(responses: (RestTaskListResponse | undefined)[], selectedStatuses: string[]): TaskStatusTabView[][] {
  return responses.map((response, index) => {
    const counts = response?.status_counts ?? {}
    const allCount = statuses.reduce((total, status) => total + (counts[status] ?? 0), 0)
    return [
      { value: GetTasksStatus.all, label: "tasks.status.all", count: allCount, selected: selectedStatuses[index] === GetTasksStatus.all },
      ...statuses.map((status) => ({ value: status, label: statusLabels[status], count: counts[status] ?? 0, selected: selectedStatuses[index] === status })),
    ]
  })
}

function formatMinutes(minutes: number | null | undefined) {
  if (minutes === null || minutes === undefined || !Number.isSafeInteger(minutes) || minutes < 0) return "—"
  return `${Math.floor(minutes / 60)}:${String(minutes % 60).padStart(2, "0")}`
}
