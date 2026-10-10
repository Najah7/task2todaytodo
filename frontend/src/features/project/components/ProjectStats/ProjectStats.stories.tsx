import type { Meta, StoryObj } from "@storybook/react-vite"
import { ProjectListStoryFrame, projectListStoryHandlers } from "~storybook/project/ProjectListStoryFrame"
import ProjectStats from "."

const meta = {
  title: "Projects/ProjectStats",
  component: ProjectStats,
  parameters: { msw: { handlers: projectListStoryHandlers } },
} satisfies Meta<typeof ProjectStats>

export default meta
type Story = StoryObj<typeof meta>

export const InProgress: Story = { render: () => <ProjectListStoryFrame><ProjectStats /></ProjectListStoryFrame> }

export const Trash: Story = {
  render: () => <ProjectListStoryFrame path="/projects?view=trash"><ProjectStats /></ProjectListStoryFrame>,
  globals: { locale: "en" },
}
