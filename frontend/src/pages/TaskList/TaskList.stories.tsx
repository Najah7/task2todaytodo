import type { Meta, StoryObj } from "@storybook/react-vite"
import { http, HttpResponse } from "msw"
import { taskListStoryHandlers, taskListStoryResponse } from "~storybook/task/handlers"
import { TaskListStoryFrame } from "~storybook/task/TaskListStoryFrame"

const meta = {
  title: "Tasks/TaskListPage",
  component: TaskListStoryFrame,
  parameters: { layout: "fullscreen", msw: { handlers: taskListStoryHandlers } },
} satisfies Meta<typeof TaskListStoryFrame>

export default meta
type Story = StoryObj<typeof meta>

export const Populated: Story = {}
export const Loading: Story = {
  parameters: { msw: { handlers: [http.get("*/api/tasks", () => new Promise(() => {})), ...taskListStoryHandlers.slice(1)] } },
}
export const Empty: Story = {
  parameters: { msw: { handlers: [http.get("*/api/tasks", () => HttpResponse.json({ ...taskListStoryResponse, items: [], next_page_token: "", total_count: 0 })), ...taskListStoryHandlers.slice(1)] } },
}
export const Error: Story = {
  parameters: { msw: { handlers: [http.get("*/api/tasks", () => HttpResponse.json({ error: { message: "Storybook task-list error" } }, { status: 500 })), ...taskListStoryHandlers.slice(1)] } },
}
export const ProjectOptionsError: Story = {
  parameters: { msw: { handlers: [taskListStoryHandlers[0]!, http.get("*/api/projects", () => HttpResponse.json({ error: { message: "Storybook project-options error" } }, { status: 500 })), ...taskListStoryHandlers.slice(2)] } },
}
