import type { Meta, StoryObj } from "@storybook/react-vite"
import { withProjectListStoryContext } from "~storybook/project/ProjectListStoryProvider"
import ProjectStats from "."

const meta = {
  title: "Projects/ProjectStats",
  component: ProjectStats,
  decorators: [withProjectListStoryContext()],
} satisfies Meta<typeof ProjectStats>

export default meta
type Story = StoryObj<typeof meta>

export const InProgress: Story = {}

export const Trash: Story = { decorators: [withProjectListStoryContext({ navigation: { state: { tab: "trash" } } })], globals: { locale: "en" } }
