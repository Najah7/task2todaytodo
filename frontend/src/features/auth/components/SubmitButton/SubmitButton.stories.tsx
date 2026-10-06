import type { Meta, StoryObj } from "@storybook/react-vite"
import SubmitButton from "./index"

const meta = {
  title: "Auth/SubmitButton",
  component: SubmitButton,
  args: {
    submitting: false,
    children: "Log in",
    pendingLabel: "Logging in…",
  },
} satisfies Meta<typeof SubmitButton>

export default meta
type Story = StoryObj<typeof meta>

export const Default: Story = {}
export const Submitting: Story = { args: { submitting: true } }
