import { http, HttpResponse } from "msw"
import type { Meta, StoryObj } from "@storybook/react-vite"
import { ProjectListStoryFrame, projectListStoryHandlers, projectListStoryOptions, projectListStoryProject, projectListStoryResponse } from "~storybook/project/ProjectListStoryFrame"
import ProjectPaginationFooter from "."

const meta = {
  title: "Projects/ProjectPaginationFooter",
  component: ProjectPaginationFooter,
  parameters: { msw: { handlers: projectListStoryHandlers } },
} satisfies Meta<typeof ProjectPaginationFooter>

export default meta
type Story = StoryObj<typeof meta>

const paginationHandlers = (response: ReturnType<typeof projectListStoryResponse>) => [
  http.get("*/api/projects", () => HttpResponse.json(response)),
  http.get("*/api/projects/options", () => HttpResponse.json(projectListStoryOptions)),
]

export const Enabled: Story = {
  render: () => <ProjectListStoryFrame path="/projects?status=in_progress&page_token=middle"><ProjectPaginationFooter /></ProjectListStoryFrame>,
  parameters: { msw: { handlers: paginationHandlers(projectListStoryResponse([projectListStoryProject], { previous: "previous", next: "next" })) } },
}

export const FirstPageRecovery: Story = {
  render: () => <ProjectListStoryFrame path="/projects?status=in_progress&page_token=stale"><ProjectPaginationFooter /></ProjectListStoryFrame>,
  parameters: { msw: { handlers: paginationHandlers(projectListStoryResponse([])) } },
}

export const PreviousDisabled: Story = {
  render: () => <ProjectListStoryFrame><ProjectPaginationFooter /></ProjectListStoryFrame>,
  parameters: { msw: { handlers: paginationHandlers(projectListStoryResponse([projectListStoryProject], { next: "next" })) } },
}
