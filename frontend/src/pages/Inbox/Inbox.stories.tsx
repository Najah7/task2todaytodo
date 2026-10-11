import type { Meta, StoryObj } from "@storybook/react-vite"
import { http, HttpResponse } from "msw"
import { InboxStoryFrame } from "~storybook/inbox/InboxStoryFrame"
import { inboxStoryHandlers, inboxStoryResponse } from "~storybook/inbox/handlers"

const meta = {
  title: "Inbox/InboxPage",
  component: InboxStoryFrame,
  parameters: { layout: "fullscreen", msw: { handlers: inboxStoryHandlers } },
} satisfies Meta<typeof InboxStoryFrame>

export default meta
type Story = StoryObj<typeof meta>

export const Populated: Story = {}
export const Empty: Story = {
  parameters: { msw: { handlers: [http.get("*/api/inbox", () => HttpResponse.json({ ...inboxStoryResponse, items: [], counts: { all: 0, task: 0, schedule: 0 }, next_page_token: "", previous_page_token: "" })), ...inboxStoryHandlers.slice(1)] } },
}
export const Error: Story = {
  parameters: { msw: { handlers: [http.get("*/api/inbox", () => HttpResponse.json({ error: { message: "Storybook inbox error" } }, { status: 500 })), ...inboxStoryHandlers.slice(1)] } },
}
