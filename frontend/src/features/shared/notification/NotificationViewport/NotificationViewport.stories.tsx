import type { Meta, StoryObj } from "@storybook/react-vite"
import { userEvent, within } from "storybook/test"
import { notify } from "../index"
import NotificationViewport from "./index"

function NotificationPreview() {
  return (
    <div style={{ display: "grid", gap: "1rem", padding: "2rem", minHeight: "20rem", background: "var(--color-ground)" }}>
      <button type="button" onClick={() => notify.success("プロジェクトを保存しました。")}>成功通知を表示</button>
      <button type="button" onClick={() => notify.error("保存できませんでした。もう一度お試しください。")}>エラー通知を表示</button>
      <NotificationViewport />
    </div>
  )
}

const meta = {
  title: "Shared/Notification",
  component: NotificationPreview,
} satisfies Meta<typeof NotificationPreview>

export default meta
type Story = StoryObj<typeof meta>

export const SuccessAndError: Story = {
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement)
    await userEvent.click(canvas.getByRole("button", { name: "成功通知を表示" }))
    await userEvent.click(canvas.getByRole("button", { name: "エラー通知を表示" }))
  },
}
