import { useState } from "react"
import { QueryClient, QueryClientProvider } from "@tanstack/react-query"
import { createMemoryRouter, RouterProvider } from "react-router"
import { http, HttpResponse } from "msw"
import type { Meta, StoryObj } from "@storybook/react-vite"
import { expect, userEvent, within } from "storybook/test"
import type { RestProjectOptionsResponse, RestProjectResponse } from "~/api/generated/projects"
import App from "~/App"
import ProjectsEditPage from "~/pages/ProjectsEdit"
import ProjectsNewPage from "~/pages/ProjectsNew"

const options: RestProjectOptionsResponse = {
  types: [
    { label: "Other", label_jp: "その他", value: "other" },
    { label: "Work", label_jp: "仕事", value: "work" },
  ],
  priorities: [
    { label: "Low", label_jp: "低", value: "low", weight: 1 },
    { label: "Medium", label_jp: "中", value: "medium", weight: 2 },
    { label: "High", label_jp: "高", value: "high", weight: 3 },
  ],
  statuses: [],
}

const project: RestProjectResponse = {
  can_delete: true,
  can_update: true,
  created_at: 1,
  deleted_at: null,
  description: "Collect and summarize quarterly results.",
  end_date: "2026-10-31",
  goal: "Publish the report",
  id: "project-1",
  priority: { label: "High", label_jp: "高", value: "high", weight: 3 },
  progress: 0,
  remaining_days: 21,
  revision: 3,
  start_date: "2026-10-01",
  status: "open",
  title: "Quarterly report",
  type: { label: "Work", label_jp: "仕事", value: "work" },
  updated_at: 1,
  user_id: "user-story",
}

const standardHandlers = [
  http.get("*/api/projects/options", () => HttpResponse.json(options)),
  http.get("*/api/projects/project-1", () => HttpResponse.json(project)),
  http.post("*/api/projects", () => HttpResponse.json({ ...project, id: "project-created" })),
  http.patch("*/api/projects/project-1", () => HttpResponse.json({ ...project, revision: 4 })),
]

const meta = {
  title: "Projects/ProjectForm",
  parameters: { layout: "fullscreen", msw: { handlers: standardHandlers } },
} satisfies Meta

export default meta
type Story = Omit<StoryObj<typeof meta>, "args"> & { args?: never }

function ProjectFormPreview({ mode = "create" }: { mode?: "create" | "edit" }) {
  const [client] = useState(() => new QueryClient({ defaultOptions: { queries: { retry: false } } }))
  const [router] = useState(() => createMemoryRouter([
    { path: "/projects", element: <App><h1>Projects open</h1></App> },
    { path: "/projects/new", element: <App><ProjectsNewPage /></App> },
    { path: "/projects/:id/edit", element: <App><ProjectsEditPage /></App> },
  ], { initialEntries: [mode === "edit" ? `/projects/${project.id}/edit` : "/projects/new"] }))
  return <QueryClientProvider client={client}><RouterProvider router={router} /></QueryClientProvider>
}

export const Create: Story = { render: () => <ProjectFormPreview /> }

export const Edit: Story = { render: () => <ProjectFormPreview mode="edit" /> }

export const DateValidation: Story = {
  render: () => <ProjectFormPreview />,
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement)
    await userEvent.type(await canvas.findByLabelText(/プロジェクト名|Project name/), "Quarterly report")
    await userEvent.type(canvas.getByLabelText(/開始日|Start date/), "20261020")
    await userEvent.type(canvas.getByLabelText(/期限|Deadline/), "20261010")
    await userEvent.click(canvas.getByRole("button", { name: /作成する|Create/ }))
    await expect(await canvas.findByText(/期限は開始日以降|Deadline must be on or after/)).toBeVisible()
  },
}

export const ServerFieldValidation: Story = {
  render: () => <ProjectFormPreview />,
  parameters: {
    msw: {
      handlers: [
        ...standardHandlers.filter((_handler, index) => index !== 2),
        http.post("*/api/projects", () => HttpResponse.json({ error: { details: [{ field: "start_date", code: "invalid_date" }] } }, { status: 422 })),
      ],
    },
  },
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement)
    await userEvent.type(await canvas.findByLabelText(/プロジェクト名|Project name/), "Quarterly report")
    await userEvent.click(canvas.getByRole("button", { name: /作成する|Create/ }))
    await expect(await canvas.findByText(/有効な日付を入力してください|Enter a valid date/)).toBeVisible()
    await expect(canvas.getByLabelText(/開始日|Start date/)).toHaveAttribute("aria-invalid", "true")
  },
}

export const DirtyNavigation: Story = {
  render: () => <ProjectFormPreview />,
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement)
    await userEvent.type(await canvas.findByLabelText(/プロジェクト名|Project name/), "A draft")
    await userEvent.click(canvas.getByRole("button", { name: /キャンセル|Cancel/ }))
    await expect(await canvas.findByRole("alertdialog", { name: /変更を破棄|Discard changes/ })).toBeVisible()
    await userEvent.click(canvas.getByRole("button", { name: /編集を続ける|Keep editing/ }))
    await expect(canvas.getByLabelText(/プロジェクト名|Project name/)).toHaveValue("A draft")
  },
}

export const ConflictReload: Story = {
  render: () => <ProjectFormPreview mode="edit" />,
  parameters: {
    msw: {
      handlers: [
        http.get("*/api/projects/options", () => HttpResponse.json(options)),
        http.get("*/api/projects/project-1", () => HttpResponse.json({ ...project, title: "Latest project revision" })),
        http.patch("*/api/projects/project-1", () => HttpResponse.json({ error: { message: "Conflict", details: [{ field: "If-Match", code: "revision_conflict" }] } }, { status: 409 })),
      ],
    },
  },
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement)
    await canvas.findByDisplayValue("Latest project revision")
    await userEvent.clear(canvas.getByLabelText(/プロジェクト名|Project name/))
    await userEvent.type(canvas.getByLabelText(/プロジェクト名|Project name/), "My edit")
    await userEvent.click(canvas.getByRole("button", { name: /保存する|Save/ }))
    const reloadButton = await canvas.findByRole("button", { name: /最新の内容を読み込む|Load latest content/ })
    await expect(canvas.getByText(/競合|別のユーザーが変更|conflict|another user/i)).toBeVisible()
    await userEvent.click(reloadButton)
    const dialog = await canvas.findByRole("alertdialog", { name: /最新の内容を読み込みますか|Load the latest content/ })
    await userEvent.click(within(dialog).getByRole("button", { name: /最新の内容を読み込む|Load latest content/ }))
    await expect(canvas.getByLabelText(/プロジェクト名|Project name/)).toHaveValue("Latest project revision")
  },
}
