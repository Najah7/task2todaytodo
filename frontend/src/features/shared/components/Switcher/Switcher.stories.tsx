import type { Meta, StoryObj } from "@storybook/react-vite"
import { useArgs } from "storybook/preview-api"
import { fn } from "storybook/test"
import Switcher from "./index"

const meta = {
  title: "Shared/Switcher",
  component: Switcher,
  args: {
    label: "Display language",
    leftLabel: "English",
    rightLabel: "日本語",
    left: "EN",
    right: "JP",
    rightSelected: false,
    disabled: false,
    onChange: fn(),
  },
  render: function Render(args) {
    const [, updateArgs] = useArgs()
    return <Switcher {...args} onChange={(rightSelected) => {
      args.onChange(rightSelected)
      updateArgs({ rightSelected })
    }} />
  },
} satisfies Meta<typeof Switcher>

export default meta
type Story = StoryObj<typeof meta>

export const English: Story = {}
export const Japanese: Story = { args: { rightSelected: true } }
export const Disabled: Story = { args: { disabled: true } }
