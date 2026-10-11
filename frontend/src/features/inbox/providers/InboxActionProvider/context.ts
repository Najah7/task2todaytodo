import { createContext } from "react"
import type { RestInboxItemResponse } from "~/api/generated/inbox"
import type { InboxQuickCreateValues } from "~/features/inbox/components/InboxQuickCreate/schema"

export type InboxTaskCreateInput = { title: string; projectId: string }

export type InboxActions = {
  busy: boolean
  createTask: (input: InboxTaskCreateInput) => Promise<void>
  createSchedule: (values: InboxQuickCreateValues) => Promise<void>
  complete: (item: RestInboxItemResponse) => Promise<void>
  retry: () => Promise<void>
  retryOptions: () => Promise<void>
}

export const InboxActionContext = createContext<InboxActions | null>(null)
