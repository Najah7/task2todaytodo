import type { MessageKey } from "~/features/i18n/messages/types"

export type TodayBoardItem = {
  id: string
  kind: "task" | "action_item" | "schedule"
  title: string
  groupLabel: string
  status: MessageKey
  timeLabel: string
  durationLabel: string
  completed: boolean
  history: boolean
  canComplete: boolean
  canReopen: boolean
  canMove: boolean
  canRemove: boolean
  canHide: boolean
}

export type TodayBoardProps = {
  dateLabel: string
  items: TodayBoardItem[]
  completedCount: number
  totalCount: number
  totalTimeLabel: string
  completedTimeLabel: string
  remainingTimeLabel: string
  nextSchedule?: { title: string; startLabel: string; endLabel: string; countdownLabel: string }
  hiddenScheduleCount: number
  busy?: boolean
  onComplete: (itemId: string) => void
  onReopen: (itemId: string) => void
  onMove: (itemId: string, direction: -1 | 1) => void
  onReorder: (itemId: string, targetItemId: string, position: "before" | "after") => void
  onRemove: (itemId: string) => void
  onHideSchedule: (itemId: string) => void
  onShowHiddenSchedules: () => void
  onOpenHelp: () => void
  onRegenerate: () => void
  onRegenerateWithoutAllocations?: () => void
}
