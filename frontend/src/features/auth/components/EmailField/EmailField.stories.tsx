import type { ComponentProps } from "react"
import type { Meta, StoryObj } from "@storybook/react-vite"
import { useForm } from "react-hook-form"
import { withFieldWidth } from "~storybook/auth/decorators"
import EmailField from "./index"

function EmailFieldPreview(args: Omit<ComponentProps<typeof EmailField>, "registration">) {
  const { register } = useForm({ defaultValues: { email: "member@example.com" } })
  return <EmailField {...args} registration={register("email")} />
}

const meta = {
  title: "Auth/EmailField",
  component: EmailFieldPreview,
  decorators: [withFieldWidth],
  args: { readOnly: false },
} satisfies Meta<typeof EmailFieldPreview>

export default meta
type Story = StoryObj<typeof meta>

export const Default: Story = {}
export const Invalid: Story = { args: { error: "auth.error.invalidEmail" } }
export const ReadOnly: Story = { args: { readOnly: true } }
