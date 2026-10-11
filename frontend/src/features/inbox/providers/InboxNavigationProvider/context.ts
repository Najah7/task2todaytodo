import { createContext } from "react"
import type { InboxListState } from "~/features/inbox/converters/inboxSearchParams"
import type { RestInboxItemResponse } from "~/api/generated/inbox"

export type InboxNavigation = {
  state: InboxListState
  currentURL: string
  selectType: (type: InboxListState["type"]) => void
  selectProject: (projectId: string) => void
  changeTitle: (title: string) => void
  first: () => void
  previous: (pageToken?: string) => void
  next: (pageToken?: string) => void
  editTask: (taskId: string) => void
  editSchedule: (blueprintId: string, occurrenceDate: string) => void
  editItem: (item: RestInboxItemResponse) => void
}

export const InboxNavigationContext = createContext<InboxNavigation | null>(null)
