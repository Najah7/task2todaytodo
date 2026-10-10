import type { Meta, StoryObj } from "@storybook/react-vite"
import { ProjectListStoryFrame, projectListStoryHandlers } from "~storybook/project/ProjectListStoryFrame"
import ProjectStatusTab from "."

const meta = {
  title: "Projects/ProjectStatusTab",
  component: ProjectStatusTab,
  parameters: { msw: { handlers: projectListStoryHandlers } },
} satisfies Meta<typeof ProjectStatusTab>

export default meta
type Story = StoryObj<typeof meta>

export const InProgress: Story = { render: () => <ProjectListStoryFrame><ProjectStatusTab /></ProjectListStoryFrame> }

export const Trash: Story = {
  render: () => <ProjectListStoryFrame path="/projects?view=trash"><ProjectStatusTab /></ProjectListStoryFrame>,
  globals: { locale: "en" },
}
