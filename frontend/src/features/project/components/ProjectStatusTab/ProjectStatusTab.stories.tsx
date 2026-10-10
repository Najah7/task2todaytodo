import type { Meta, StoryObj } from "@storybook/react-vite"
import { withProjectListStoryContext } from "~storybook/project/ProjectListStoryProvider"
import ProjectStatusTab from "."

const meta = {
  title: "Projects/ProjectStatusTab",
  component: ProjectStatusTab,
  decorators: [withProjectListStoryContext()],
} satisfies Meta<typeof ProjectStatusTab>

export default meta
type Story = StoryObj<typeof meta>

export const InProgress: Story = {}

export const Trash: Story = { decorators: [withProjectListStoryContext({ navigation: { state: { tab: "trash" } } })] }
