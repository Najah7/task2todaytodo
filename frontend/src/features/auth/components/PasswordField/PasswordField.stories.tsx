import type { ComponentProps } from "react"
import type { Meta, StoryObj } from "@storybook/react-vite"
import { useForm } from "react-hook-form"
import { useArgs } from "storybook/preview-api"
import { useI18n } from "~/features/i18n/hooks"
import { withFieldWidth } from "~storybook/auth/decorators"
import PasswordField from "./index"

type StoryArgs = Omit<ComponentProps<typeof PasswordField>, "registration">

function PasswordFieldPreview(args: StoryArgs) {
  const { register } = useForm({ defaultValues: { password: "Password1!" } })
  const i18n = useI18n()
  return <PasswordField {...args} help={args.help ?? i18n("auth.password.help")} registration={register("password")} />
}

const meta = {
  title: "Auth/PasswordField",
  component: PasswordFieldPreview,
  decorators: [withFieldWidth],
  args: { visible: false, autoComplete: "new-password", readOnly: false },
  render: function Render(args) {
    const [, updateArgs] = useArgs<StoryArgs>()
    return <PasswordFieldPreview {...args} onVisibilityChange={(visible) => updateArgs({ visible })} />
  },
} satisfies Meta<typeof PasswordFieldPreview>

export default meta
type Story = StoryObj<typeof meta>

export const Default: Story = {}
export const Visible: Story = { args: { visible: true } }
export const Invalid: Story = { args: { error: "auth.error.invalidPassword" } }
export const ReadOnly: Story = { args: { readOnly: true } }
