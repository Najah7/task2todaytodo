export type TaskActionItemRowView = {
  occurrenceKey: string
  taskId: string
  actionItemId: string
  seriesId?: string
  occurrenceDate?: string
  title: string
  estimateLabel: string
  completed: boolean
  occurrenceDateLabel: string
  todayRegistration: { timeLabel: string } | null
}

export type TaskListRowView = {
  id: string
  title: string
  canUpdate: boolean
  projectLabel: string
  statusLabel: string
  statusTone: "open" | "active" | "waiting" | "done"
  actionItemProgressLabel: string
  actionItemCount: number
  actionItemCompletedCount: number
  progress: number
  estimateLabel: string
  dueDateLabel: string
  dueDateTone: "normal" | "today" | "overdue" | "none"
  actionItems: TaskActionItemRowView[]
}

export type TaskActionItemsQueryState = "loading" | "error" | "success"
