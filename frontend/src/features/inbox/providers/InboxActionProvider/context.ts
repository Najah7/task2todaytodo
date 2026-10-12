import { createContext } from "react"
import type { RestInboxItemResponse } from "~/api/generated/inbox"
import type { RestActionItemResponse } from "~/api/generated/tasks"
import type { InboxQuickCreateValues } from "~/features/inbox/components/InboxQuickCreate/schema"

export type InboxTaskCreateInput = { title: string; projectId: string; estimatedMinutes: number }

export type InboxActions = {
  busy: boolean
  createTask: (input: InboxTaskCreateInput) => Promise<void>
  createSchedule: (values: InboxQuickCreateValues) => Promise<void>
  complete: (item: RestInboxItemResponse, actualMinutes?: number) => Promise<void>
  completeActionItem: (taskId: string, item: RestActionItemResponse, actualMinutes?: number) => Promise<void>
  reopenActionItem: (taskId: string, item: RestActionItemResponse) => Promise<void>
  addActionItemToToday: (taskId: string, item: RestActionItemResponse) => Promise<void>
  addTaskToToday: (taskId: string, priority?: string) => Promise<void>
  retry: () => Promise<void>
  retryOptions: () => Promise<void>
}

export const InboxActionContext = createContext<InboxActions | null>(null)
