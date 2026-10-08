import { useState } from "react"
import { expect, userEvent, within } from "storybook/test"
import type { Meta, StoryObj } from "@storybook/react-vite"
import { useI18n, useLanguage } from "~/features/i18n/hooks"
import SeachSelect, { type SeachSelectOption } from "."

const statuses: Record<"ja" | "en", SeachSelectOption[]> = {
  ja: [
    { value: "in_progress", label: "進行中" },
    { value: "pending", label: "保留" },
    { value: "done", label: "完了" },
    { value: "open", label: "オープン" },
    { value: "waiting_on_others", label: "他者待ち" },
  ],
  en: [
    { value: "in_progress", label: "In progress" },
    { value: "pending", label: "Pending" },
    { value: "done", label: "Done" },
    { value: "open", label: "Open" },
    { value: "waiting_on_others", label: "Waiting on others" },
  ],
}

const manyOptions = Array.from({ length: 12 }, (_, index) => ({
  value: `choice-${index + 1}`,
  label: `Choice ${index + 1}`,
}))

function Preview({
  options,
  disabled = false,
}: {
  options?: SeachSelectOption[]
  disabled?: boolean
}) {
  const i18n = useI18n()
  const { language } = useLanguage()
  const choices = options ?? statuses[language]
  const [value, setValue] = useState(choices[0].value)
  const selectedLabel = choices.find((option) => option.value === value)?.label ?? value

  return (
    <div style={{ width: "24rem", padding: "var(--space-lg)", background: "var(--color-ground)" }}>
      <label id="story-status-label" className="text-field-label" htmlFor="story-status">
        {i18n("projects.column.status")}
      </label>
      <SeachSelect
        id="story-status"
        labelledBy="story-status-label"
        value={value}
        options={choices}
        onChange={setValue}
        disabled={disabled}
      />
      <p className="text-caption">{i18n("projects.column.status")}: {selectedLabel}</p>
    </div>
  )
}

const meta = {
  title: "Shared/SeachSelect",
  component: Preview,
  parameters: { layout: "centered" },
  render: () => <Preview />,
} satisfies Meta<typeof Preview>

export default meta
type Story = StoryObj<typeof meta>

export const Default: Story = {}

export const OpenPopup: Story = {
  play: async ({ canvasElement }) => {
    await userEvent.click(within(canvasElement).getByRole("button"))
    await expect(within(document.body).findByRole("listbox")).resolves.toBeVisible()
  },
}

export const SearchNoResults: Story = {
  play: async ({ canvasElement }) => {
    await userEvent.click(within(canvasElement).getByRole("button"))
    const search = await within(document.body).findByRole("combobox")
    await userEvent.type(search, "not-present")
    await expect(within(document.body).findByRole("status")).resolves.toHaveTextContent(/該当する項目はありません。|No matching options\./)
  },
}

export const ScrollableOptions: Story = {
  render: () => <Preview options={manyOptions} />,
  play: async ({ canvasElement }) => {
    await userEvent.click(within(canvasElement).getByRole("button"))
    await expect(within(document.body).findByRole("listbox")).resolves.toBeVisible()
  },
}

export const Disabled: Story = {
  render: () => <Preview disabled />,
}
