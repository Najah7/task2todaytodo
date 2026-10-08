import { expect, test } from "vitest"
import type { RestProjectListResponse, RestProjectResponse } from "~/api/generated/projects"
import { removeStatusChangedProject } from "./listCache"

const project: RestProjectResponse = {
  can_delete: true,
  can_update: true,
  created_at: 1,
  deleted_at: null,
  description: "",
  end_date: "2026-10-02",
  goal: "",
  id: "p-1",
  priority: { label: "High", label_jp: "高", value: "high", weight: 3 },
  progress: 25,
  remaining_days: -7,
  revision: 4,
  start_date: null,
  status: "in_progress",
  title: "Project",
  type: { label: "Other", label_jp: "その他", value: "other" },
  updated_at: 1,
  user_id: "u-1",
}

const list: RestProjectListResponse = {
  items: [project],
  next_page_token: "next",
  previous_page_token: "previous",
  summary: {
    due_soon_count: 2,
    overdue_count: 3,
    status_counts: { done: 2, in_progress: 4, open: 5, pending: 1, waiting_on_others: 1 },
    timezone: "Asia/Tokyo",
    today: "2026-10-09",
    total_count: 4,
    trash_count: 0,
  },
}

test("optimistically removes a status-changed row and updates its whole-tab counters", () => {
  const next = removeStatusChangedProject(list, project, "pending")
  expect(next.items).toEqual([])
  expect(next.summary.total_count).toBe(3)
  expect(next.summary.overdue_count).toBe(3)
  expect(next.summary.due_soon_count).toBe(2)
  expect(next.summary.status_counts).toEqual({ done: 2, in_progress: 3, open: 5, pending: 2, waiting_on_others: 1 })
  expect(next.next_page_token).toBe("next")
  expect(next.previous_page_token).toBe("previous")
})
