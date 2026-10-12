import { http, HttpResponse } from "msw"
import type { RestInboxListResponse } from "~/api/generated/inbox"

export const inboxStoryResponse: RestInboxListResponse = {
  counts: { all: 3, task: 2, schedule: 1 },
  items: [
    {
      id: "task-1",
      type: "task",
      title: "通知設定を調べる",
      project_id: "project-1",
      project_name: "Webサイト改善",
      created_at: "2026-10-11T08:00:00Z",
      task: {
        id: "task-1", assignee_id: "user-story", can_assign: true, can_update: true,
        description: "", due_date: "2026-10-15", estimated_minutes: 90, actual_minutes: null,
        estimate_source: "manual", priority: "medium", progress: 0, remaining_days: 3,
        action_item_completed_count: 0, action_item_count: 2, revision: 1, status: "open",
      },
      schedule: null,
    },
    {
      id: "schedule-1",
      type: "schedule",
      title: "歯医者",
      project_id: null,
      project_name: null,
      created_at: "2026-10-10T08:00:00Z",
      task: null,
      schedule: {
        all_day: false, assignee_id: "user-story", rule_id: "schedule-1", can_assign: true,
        can_update: true, can_update_following: true, completed: false, description: "", end_at: "2026-10-13T10:30:00Z",
        location: "", occurrence_date: "2026-10-13", occurrence_id: null, revision: 1, skipped: false,
        source_revision_id: "revision-1", start_at: "2026-10-13T09:30:00Z", timezone: "Asia/Tokyo",
      },
    },
    {
      id: "task-2",
      type: "task",
      title: "領収書を集める",
      project_id: null,
      project_name: null,
      created_at: "2026-10-09T08:00:00Z",
      task: {
        id: "task-2", assignee_id: "user-story", can_assign: true, can_update: false,
        description: "", due_date: null, estimated_minutes: null, actual_minutes: null,
        estimate_source: "action_items", priority: "low", progress: 0, remaining_days: null,
        action_item_completed_count: 0, action_item_count: 1, revision: 1, status: "open",
      },
      schedule: null,
    },
  ],
  next_page_token: "next-page",
  previous_page_token: "",
}

const projectListResponse = {
  items: [{
    can_delete: true, can_update: true, created_at: 1, deleted_at: null, description: "", end_date: null,
    goal: "", id: "project-1", priority: { label: "Medium", label_jp: "中", value: "medium", weight: 2 },
    progress: 0, remaining_days: null, revision: 1, start_date: null, status: "in_progress",
    title: "Webサイト改善", type: { label: "Other", label_jp: "その他", value: "other" }, updated_at: 1, user_id: "user-story",
  }],
  next_page_token: "",
  previous_page_token: "",
  summary: { due_soon_count: 0, overdue_count: 0, status_counts: { done: 0, in_progress: 1, open: 0, pending: 0, waiting_on_others: 0 }, timezone: "Asia/Tokyo", today: "2026-10-12", total_count: 1, trash_count: 0 },
}

export const inboxStoryHandlers = [
  http.get("*/api/inbox", () => HttpResponse.json(inboxStoryResponse)),
  http.get("*/api/projects", () => HttpResponse.json(projectListResponse)),
  http.get("*/api/users/me", () => HttpResponse.json({ user_id: "user-story", timezone: "Asia/Tokyo", email: "story@example.test", user_name: "Story" })),
  http.post("*/api/tasks", () => HttpResponse.json({ id: "task-created" }, { status: 201 })),
  http.post("*/api/projects/:projectId/tasks", () => HttpResponse.json({ id: "task-created" }, { status: 201 })),
  http.post("*/api/schedule-rules", () => HttpResponse.json({ id: "schedule-created" }, { status: 201 })),
  http.post(/\/api\/tasks\/[^/]+:complete$/, () => HttpResponse.json({ message: "Task completed" })),
  http.post(/\/api\/schedules\/[^/]+\/[^/]+:complete$/, () => new HttpResponse(null, { status: 204 })),
]
