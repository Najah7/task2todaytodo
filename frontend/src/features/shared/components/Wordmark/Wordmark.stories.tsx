import type { Meta, StoryObj } from "@storybook/react-vite"
import Wordmark from "./index"

const meta = {
  title: "Shared/Wordmark",
  component: Wordmark,
  args: { width: 176, height: 22 },
} satisfies Meta<typeof Wordmark>

export default meta
type Story = StoryObj<typeof meta>

export const Default: Story = {}
