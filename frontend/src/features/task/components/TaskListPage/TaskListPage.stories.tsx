import type { Meta, StoryObj } from "@storybook/react-vite"
import TaskListPage from "."
import { TaskListProvider } from "~/features/task/providers/TaskList"
import type { TaskActionItemRowView, TaskFilterView, TaskListData } from "~/features/task/types"

const rows: TaskListData["rows"] = [
  {
    id: "task-1", title: "オンボーディングを改善する", canUpdate: true, projectLabel: "Webサイト改善", statusLabel: "進行中", statusTone: "active",
    actionItemProgressLabel: "2 / 5", actionItemCount: 5, actionItemCompletedCount: 2, progress: 40, estimateLabel: "3:30", dueDateLabel: "10月30日（金）", dueDateTone: "normal",
    actionItems: [
      { occurrenceKey: "item-1:2026-10-01", taskId: "task-1", actionItemId: "item-1", title: "離脱ポイントを洗い出す", estimateLabel: "0:30", completed: true, occurrenceDate: "2026-10-01", occurrenceDateLabel: "", todayRegistration: null },
      { occurrenceKey: "item-2:2026-10-02", taskId: "task-1", actionItemId: "item-2", title: "競合のオンボーディングを調べる", estimateLabel: "0:45", completed: true, occurrenceDate: "2026-10-02", occurrenceDateLabel: "", todayRegistration: null },
      { occurrenceKey: "series-1:2026-10-10", taskId: "task-1", actionItemId: "series-1", seriesId: "series-1", title: "導入画面の構成を考える", estimateLabel: "1:00", completed: false, occurrenceDate: "2026-10-10", occurrenceDateLabel: "10月10日", todayRegistration: { timeLabel: "10:30" } },
      { occurrenceKey: "series-2:2026-10-10", taskId: "task-1", actionItemId: "series-2", seriesId: "series-2", title: "フィードバックを反映", estimateLabel: "0:30", completed: false, occurrenceDate: "2026-10-10", occurrenceDateLabel: "10月10日", todayRegistration: { timeLabel: "14:00" } },
      { occurrenceKey: "series-3:2026-10-10", taskId: "task-1", actionItemId: "series-3", seriesId: "series-3", title: "文言を見直す", estimateLabel: "0:45", completed: false, occurrenceDate: "2026-10-10", occurrenceDateLabel: "10月10日", todayRegistration: null },
    ],
  },
  {
    id: "task-2", title: "書類を提出する", canUpdate: false, projectLabel: "—", statusLabel: "未着手", statusTone: "open",
    actionItemProgressLabel: "0 / 1", actionItemCount: 1, actionItemCompletedCount: 0, progress: 0, estimateLabel: "0:15", dueDateLabel: "10月10日（土）", dueDateTone: "today",
    actionItems: [{ occurrenceKey: "item-6", taskId: "task-2", actionItemId: "item-6", title: "必要書類を確認する", estimateLabel: "0:15", completed: false, occurrenceDateLabel: "", todayRegistration: null }],
  },
]

const baseData: TaskListData = {
  tabs: [
    { value: "all", label: "tasks.status.all", count: 7, selected: true },
    { value: "open", label: "tasks.status.open", count: 4, selected: false },
    { value: "in_progress", label: "tasks.status.inProgress", count: 3, selected: false },
    { value: "pending", label: "tasks.status.pending", count: 0, selected: false },
    { value: "waiting_on_others", label: "tasks.status.waitingOnOthers", count: 0, selected: false },
    { value: "done", label: "tasks.status.done", count: 2, selected: false },
  ],
  filters: {
    projectOptions: [{ value: "", label: "すべて" }, { value: "project-1", label: "Webサイト改善" }],
    dueOptions: [
      { value: "all", label: "すべて" }, { value: "overdue", label: "期限超過" }, { value: "today", label: "今日" },
      { value: "due_soon", label: "今日から14日以内" }, { value: "no_due", label: "期限なし" },
    ],
    values: { projectId: "", dueFilter: "all", title: "" },
  },
  sort: "due_date:asc",
  sortOptions: [
    { value: "due_date:asc", label: "期限が近い順" }, { value: "due_date:desc", label: "期限が遠い順" },
    { value: "title:asc", label: "タスク名順" }, { value: "created_at:desc", label: "作成が新しい順" },
  ],
  rows,
  summary: { totalLabel: "7件", completedActionItemsLabel: "4", totalActionItemsLabel: "23", completedActionItems: 4, totalActionItems: 23, estimateTotalLabel: "15:50" },
  expandedIds: ["task-1"],
  actionItemsQueryStates: { "task-1": "success" },
  loading: false,
  error: false,
  canGoFirst: false,
  previousDisabled: true,
  nextDisabled: false,
  optionsLoading: false,
  optionsError: false,
}

function TaskListPageStory({ state = "populated" }: { state?: "populated" | "loading" | "empty" | "error" }) {
  const data = {
    ...baseData,
    rows: state === "empty" ? [] : baseData.rows,
    loading: state === "loading",
    error: state === "error",
  }
  const navigation = {
    selectStatus: () => {},
    changeFilter: (_filters: TaskFilterView) => {},
    changeSort: () => {},
    first: () => {}, previous: () => {}, next: () => {},
  }
  const actions = {
    create: () => {},
    edit: () => {},
    toggleExpanded: () => {},
    toggleActionItem: (_item: TaskActionItemRowView) => {},
    retryActionItems: () => {}, retry: () => {}, retryOptions: () => {},
  }
  return <TaskListProvider data={data} navigation={navigation} actions={actions}><TaskListPage /></TaskListProvider>
}

const meta = {
  title: "Tasks/TaskListPage",
  component: TaskListPageStory,
  parameters: { layout: "fullscreen" },
} satisfies Meta<typeof TaskListPageStory>

export default meta
type Story = StoryObj<typeof meta>

export const Populated: Story = {}
export const Loading: Story = { args: { state: "loading" } }
export const Empty: Story = { args: { state: "empty" } }
export const Error: Story = { args: { state: "error" } }
