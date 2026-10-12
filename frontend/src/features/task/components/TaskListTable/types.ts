export type TaskActionItemRowView = {
  id: string
  taskId: string
  title: string
  estimateLabel: string
  completed: boolean
  priority?: string
  todayRegistration: { timeLabel: string } | null
}

export type TaskListRowView = {
  id: string
  title: string
  canUpdate: boolean
  canAddToToday: boolean
  canAddActionItem: boolean
  estimateEditable: boolean
  started: boolean
  completed: boolean
  revision: number
  priority?: string
  todayRegistration: { timeLabel: string } | null
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
