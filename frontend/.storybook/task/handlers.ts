import { http, HttpResponse } from "msw"
import type { RestTaskListResponse } from "~/api/generated/tasks"

export const taskListStoryResponse: RestTaskListResponse = {
  action_item_completed_count: 2,
  action_item_total_count: 6,
  estimated_minutes_total: 225,
  items: [
    {
      action_item_completed_count: 2, action_item_count: 5, actual_minutes: null, assignee_id: "user-story", can_update: true,
      created_at: "2026-10-01T00:00:00Z", description: "", due_date: "2026-10-30", estimate_source: "action_items",
      estimated_minutes: 180, id: "task-1", manual_estimated_minutes: null, priority: "high", progress: 40,
      project_id: "project-1", project_name: "Webサイト改善", remaining_days: 20, revision: 1, status: "in_progress",
      title: "オンボーディングを改善する", updated_at: "2026-10-01T00:00:00Z", user_id: "user-story",
    },
    {
      action_item_completed_count: 0, action_item_count: 1, actual_minutes: null, assignee_id: "user-story", can_update: false,
      created_at: "2026-10-01T00:00:00Z", description: "", due_date: "2026-10-10", estimate_source: "manual",
      estimated_minutes: 15, id: "task-2", manual_estimated_minutes: 15, priority: "medium", progress: 0,
      project_id: null, project_name: null, remaining_days: 0, revision: 1, status: "open",
      title: "書類を提出する", updated_at: "2026-10-01T00:00:00Z", user_id: "user-story",
    },
  ],
  next_page_token: "next-page",
  previous_page_token: "",
  status_counts: { open: 4, in_progress: 3, pending: 0, waiting_on_others: 0, done: 2 },
  total_count: 9,
}

export const taskListStoryHandlers = [
  http.get("*/api/tasks", () => HttpResponse.json(taskListStoryResponse)),
  http.get("*/api/projects", () => HttpResponse.json({ items: [], next_page_token: "" })),
  http.get("*/api/tasks/task-1/action-items", () => HttpResponse.json({ items: [
    { id: "item-1", task_id: "task-1", title: "離脱ポイントを洗い出す", estimated_minutes: 30, completed: true },
    { id: "item-2", task_id: "task-1", title: "競合のオンボーディングを調べる", estimated_minutes: 45, completed: true },
    { id: "series-1", series_id: "series-1", occurrence_date: "2026-10-10", task_id: "task-1", title: "導入画面の構成を考える", estimated_minutes: 60, completed: false },
    { id: "series-2", series_id: "series-2", occurrence_date: "2026-10-10", task_id: "task-1", title: "フィードバックを反映", estimated_minutes: 30, completed: false },
    { id: "series-3", series_id: "series-3", occurrence_date: "2026-10-10", task_id: "task-1", title: "文言を見直す", estimated_minutes: 45, completed: false },
  ] })),
]
