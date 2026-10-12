import type { Meta, StoryObj } from "@storybook/react-vite"
import TodayBoard from "."
import type { TodayBoardProps } from "./types"

const sampleItems: TodayBoardProps["items"] = [
  { id: "work-1", kind: "action_item", title: "導入画面の構成を考える", groupLabel: "Webサイト改善 · デザイン刷新", status: "today.status.completed", timeLabel: "10:30", durationLabel: "1:00", completed: false, history: false, canComplete: true, canReopen: false, canMove: true, canRemove: true, canHide: false },
  { id: "schedule-1", kind: "schedule", title: "デザインレビュー", groupLabel: "Webサイト改善", status: "today.status.scheduled", timeLabel: "13:00", durationLabel: "0:30", completed: false, history: false, canComplete: false, canReopen: false, canMove: false, canRemove: false, canHide: true },
  { id: "work-2", kind: "action_item", title: "フィードバックを反映", groupLabel: "Webサイト改善 · デザイン刷新", status: "today.status.notScheduled", timeLabel: "14:00", durationLabel: "0:30", completed: false, history: false, canComplete: true, canReopen: false, canMove: true, canRemove: true, canHide: false },
  { id: "work-3", kind: "action_item", title: "インタビューのメモを整理する", groupLabel: "プロダクトリサーチ", status: "today.status.completed", timeLabel: "09:30", durationLabel: "0:30", completed: true, history: false, canComplete: false, canReopen: true, canMove: false, canRemove: false, canHide: false },
  { id: "history-1", kind: "action_item", title: "参考記事を読む", groupLabel: "個人", status: "today.status.history", timeLabel: "—", durationLabel: "0:15", completed: true, history: true, canComplete: false, canReopen: false, canMove: false, canRemove: false, canHide: false },
]

const baseProps: TodayBoardProps = {
  dateLabel: "2026年10月12日（月）",
  items: sampleItems,
  completedCount: 1,
  totalCount: 4,
  totalTimeLabel: "2:45",
  completedTimeLabel: "0:30",
  remainingTimeLabel: "1:45",
  nextSchedule: { title: "デザインレビュー", startLabel: "13:00", endLabel: "13:30", countdownLabel: "2時間36分" },
  hiddenScheduleCount: 2,
  onComplete: () => {}, onReopen: () => {}, onMove: () => {}, onRemove: () => {}, onHideSchedule: () => {},
  onReorder: () => {}, onShowHiddenSchedules: () => {}, onRegenerate: () => {}, onOpenHelp: () => {}, onScheduleMoveAttempt: () => {},
}

const meta = {
  title: "Todo/TodayBoard",
  component: TodayBoard,
  parameters: { layout: "fullscreen" },
} satisfies Meta<typeof TodayBoard>

export default meta
type Story = StoryObj<typeof meta>

export const Populated: Story = { args: baseProps }
export const Empty: Story = { args: { ...baseProps, items: [], totalCount: 0, completedCount: 0, hiddenScheduleCount: 0, nextSchedule: undefined } }
export const CompletedSchedule: Story = { args: {
  ...baseProps,
  items: [{ id: "schedule-done", kind: "schedule", title: "Design review", groupLabel: "Website refresh", status: "today.status.completed", timeLabel: "13:00", durationLabel: "0:30", completed: true, history: false, canComplete: false, canReopen: true, canMove: false, canRemove: false, canHide: false }],
  completedCount: 1,
  totalCount: 1,
  totalTimeLabel: "0:30",
  completedTimeLabel: "0:30",
  remainingTimeLabel: "0:00",
  hiddenScheduleCount: 0,
} }
