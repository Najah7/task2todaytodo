import { useState } from "react"
import { QueryClient, QueryClientProvider } from "@tanstack/react-query"
import { createMemoryRouter, RouterProvider } from "react-router"
import { http, HttpResponse } from "msw"
import type { Meta, StoryObj } from "@storybook/react-vite"
import App from "~/App"
import type { RestProjectListResponse, RestProjectOptionsResponse, RestProjectResponse } from "~/api/generated/projects"
import ProjectList from "."

const project = (overrides: Partial<RestProjectResponse>): RestProjectResponse => ({
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
  ...overrides,
})

const projects: RestProjectResponse[] = [
  project({ id: "proj-budget", title: "Annual budget", goal: "Collect team estimates", progress: 38, end_date: "2026-10-03", remaining_days: -6, priority: { label: "Medium", label_jp: "中", value: "medium", weight: 2 } }),
  project({ id: "proj-release", title: "October release", goal: "Coordinate the launch", progress: 62, end_date: "2026-10-15", remaining_days: 6, priority: { label: "High", label_jp: "高", value: "high", weight: 3 } }),
  project({ id: "proj-research", title: "Market research", goal: "", progress: 15, end_date: null, remaining_days: null }),
]

const trashProjects = [project({
  id: "proj-archived",
  title: "Archived campaign",
  goal: "Campaign assets and notes",
  status: "pending",
  deleted_at: 1_791_481_200,
  can_update: false,
  progress: 24,
})]

const options: RestProjectOptionsResponse = {
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

function listResponse(items: RestProjectResponse[], trashCount = 1): RestProjectListResponse {
  return {
    items,
    next_page_token: "",
    previous_page_token: "",
    summary: {
      due_soon_count: 1,
      overdue_count: 1,
      status_counts: { done: 5, in_progress: 3, open: 4, pending: 2, waiting_on_others: 1 },
      timezone: "Asia/Tokyo",
      today: "2026-10-09",
      total_count: items.length,
      trash_count: trashCount,
    },
  }
}

const handlers = [
  http.get("*/api/projects", ({ request }) => {
    const url = new URL(request.url)
    const items = url.searchParams.get("view") === "trash"
      ? trashProjects
      : projects.filter((item) => item.status === url.searchParams.get("status"))
    return HttpResponse.json(listResponse(items))
  }),
  http.get("*/api/projects/options", () => HttpResponse.json(options)),
]

const meta = {
  title: "Pages/ProjectList",
  component: ProjectList,
  parameters: { layout: "fullscreen", msw: { handlers } },
} satisfies Meta<typeof ProjectList>

export default meta
type Story = StoryObj<typeof meta>

function ProjectListPreview({ path = "/projects?status=in_progress" }: { path?: string }) {
  const [client] = useState(() => new QueryClient({ defaultOptions: { queries: { retry: false } } }))
  const [router] = useState(() => createMemoryRouter([
    { path: "/projects", element: <App><ProjectList /></App> },
  ], { initialEntries: [path] }))
  return <QueryClientProvider client={client}><RouterProvider router={router} /></QueryClientProvider>
}

export const InProgress: Story = {
  render: () => <ProjectListPreview />,
}

export const Trash: Story = {
  render: () => <ProjectListPreview path="/projects?view=trash" />,
}
