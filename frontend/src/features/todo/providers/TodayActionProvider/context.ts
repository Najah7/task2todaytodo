import { createContext } from "react"

export type TodayActions = {
  busy: boolean
  complete: (itemId: string, actualMinutes?: number) => Promise<boolean>
  reopen: (itemId: string) => Promise<boolean>
  remove: (itemId: string) => Promise<boolean>
  hide: (itemId: string) => Promise<boolean>
  restore: (itemId: string) => Promise<boolean>
  addActionItem: (actionItemId: string) => Promise<boolean>
  reorder: (itemIds: string[]) => Promise<boolean>
  regenerate: () => Promise<boolean>
}

export const TodayActionContext = createContext<TodayActions | null>(null)
