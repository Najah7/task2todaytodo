import { createContext } from "react"
import type { RestTodoTodayResponse } from "~/api/generated/todo"

export type TodayAPIData = {
  state: "loading" | "error" | "ready"
  todo?: RestTodoTodayResponse
  timezone: string
  refetch: () => Promise<unknown>
}

export const TodayAPIDataContext = createContext<TodayAPIData | null>(null)
