import type { Meta, StoryObj } from "@storybook/react-vite"
import { withProjectListStoryContext } from "~storybook/project/ProjectListStoryProvider"
import ProjectPaginationFooter from "."

const meta = {
  title: "Projects/ProjectPaginationFooter",
  component: ProjectPaginationFooter,
  decorators: [withProjectListStoryContext()],
} satisfies Meta<typeof ProjectPaginationFooter>

export default meta
type Story = StoryObj<typeof meta>

export const Enabled: Story = {}

export const FirstPageRecovery: Story = { decorators: [withProjectListStoryContext({ navigation: { showFirstPage: true, previousDisabled: true, nextDisabled: true } })] }

export const PreviousDisabled: Story = { decorators: [withProjectListStoryContext({ navigation: { previousDisabled: true } })] }
