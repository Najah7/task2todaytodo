import type { Meta, StoryObj } from "@storybook/react-vite"
import ProjectCreateButton from "."

const meta = {
  title: "Projects/ProjectCreateButton",
  component: ProjectCreateButton,
  args: { label: "プロジェクトを作成", onClick: () => {} },
} satisfies Meta<typeof ProjectCreateButton>

export default meta
type Story = StoryObj<typeof meta>

export const Default: Story = {}
