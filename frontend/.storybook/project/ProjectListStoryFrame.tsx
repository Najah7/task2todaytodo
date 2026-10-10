/* oxlint-disable react/only-export-components -- Storybook fixture exports a wrapper component and typed data. */
import { useState, type ReactNode } from "react"
import { QueryClient, QueryClientProvider } from "@tanstack/react-query"
import { createMemoryRouter, RouterProvider } from "react-router"
import { http, HttpResponse } from "msw"
import type { RestProjectListResponse, RestProjectOptionsResponse, RestProjectResponse } from "~/api/generated/projects"
import { ProjectAPIDataProvider } from "~/features/project/providers/ProjectAPIDataProvider"
import { ProjectActionProvider } from "~/features/project/providers/ProjectActionProvider"
import { ProjectNavigationProvider } from "~/features/project/providers/ProjectNavigationProvider"
import { NotificationViewport } from "~/features/shared/notification"

export const projectListStoryProject: RestProjectResponse = {
  can_delete: true,
  can_update: true,
  created_at: 1_791_481_200,
  deleted_at: null,
  description: "Prepare the next release with the product team.",
  end_date: "2026-10-15",
  goal: "Ship the October release",
  id: "proj-october-release",
  priority: { label: "High", label_jp: "高", value: "high", weight: 3 },
  progress: 62,
  remaining_days: 6,
  revision: 3,
  start_date: "2026-09-01",
  status: "in_progress",
  title: "October release",
  type: { label: "Work", label_jp: "仕事", value: "work" },
  updated_at: 1_791_481_200,
  user_id: "user-story",
}

export const projectListStoryOptions: RestProjectOptionsResponse = {
  types: [
    { label: "Work", label_jp: "仕事", value: "work" },
    { label: "Personal", label_jp: "個人", value: "personal" },
    { label: "Other", label_jp: "その他", value: "other" },
  ],
  priorities: [
    { label: "Low", label_jp: "低", value: "low", weight: 1 },
    { label: "Medium", label_jp: "中", value: "medium", weight: 2 },
    { label: "High", label_jp: "高", value: "high", weight: 3 },
  ],
  statuses: [
    { label: "In progress", label_jp: "進行中", value: "in_progress" },
    { label: "Pending", label_jp: "保留", value: "pending" },
    { label: "Done", label_jp: "完了", value: "done" },
    { label: "Open", label_jp: "オープン", value: "open" },
    { label: "Waiting on others", label_jp: "他者待ち", value: "waiting_on_others" },
  ],
}

const trashProject: RestProjectResponse = {
  ...projectListStoryProject,
  id: "proj-archived",
  title: "Archived campaign",
  status: "pending",
  deleted_at: 1_791_481_200,
  can_update: false,
}

export function projectListStoryResponse(items: RestProjectResponse[], tokens: { previous?: string; next?: string } = {}): RestProjectListResponse {
  return {
    items,
    next_page_token: tokens.next ?? "",
    previous_page_token: tokens.previous ?? "",
    summary: {
      due_soon_count: 2,
      overdue_count: 1,
      status_counts: { done: 8, in_progress: 3, open: 4, pending: 2, waiting_on_others: 1 },
      timezone: "Asia/Tokyo",
      today: "2026-10-10",
      total_count: items.length,
      trash_count: 5,
    },
  }
}

export const projectListStoryHandlers = [
  http.get("*/api/projects", ({ request }) => {
    const url = new URL(request.url)
    return HttpResponse.json(url.searchParams.get("view") === "trash"
      ? projectListStoryResponse([trashProject])
      : projectListStoryResponse([projectListStoryProject]))
  }),
  http.get("*/api/projects/options", () => HttpResponse.json(projectListStoryOptions)),
]

export function ProjectListStoryFrame({ children, path = "/projects?status=in_progress" }: { children: ReactNode; path?: string }) {
  const [client] = useState(() => new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } }))
  const [router] = useState(() => createMemoryRouter([
    { path: "/projects", element: (
      <ProjectNavigationProvider>
        <ProjectAPIDataProvider list includeOptions>
          <ProjectActionProvider>{children}</ProjectActionProvider>
        </ProjectAPIDataProvider>
      </ProjectNavigationProvider>
    ) },
  ], { initialEntries: [path] }))
  return (
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
      <NotificationViewport />
    </QueryClientProvider>
  )
}
