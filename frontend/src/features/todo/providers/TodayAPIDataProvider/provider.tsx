import { useQuery } from "@tanstack/react-query"
import { getGetTodayTodoQueryOptions } from "~/api/generated/todo"
import { TodayAPIDataContext } from "./context"
import type { ReactNode } from "react"

type Props = { children: ReactNode }

export function TodayAPIDataProvider({ children }: Props) {
  const todoQuery = useQuery({ ...getGetTodayTodoQueryOptions(), retry: false, refetchInterval: 60_000, refetchOnWindowFocus: true })
  const state = todoQuery.isPending ? "loading" : todoQuery.isError ? "error" : "ready"
  return <TodayAPIDataContext.Provider value={{ state, todo: todoQuery.data, timezone: todoQuery.data?.timezone || "UTC", refetch: todoQuery.refetch }}>
    {children}
  </TodayAPIDataContext.Provider>
}
