import type { Meta, StoryObj } from "@storybook/react-vite"
import PageHeading from "./index"

const meta = {
  title: "Shared/PageHeading",
  component: PageHeading,
} satisfies Meta<typeof PageHeading>

export default meta
type Story = StoryObj<typeof meta>

export const TitleOnly: Story = { args: { messageKey: "page.today.title" } }
export const WithDescription: Story = {
  args: { messageKey: "page.projects.title", descriptionKey: "projects.description" },
}
