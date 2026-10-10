import { useState } from "react"
import { QueryClient, QueryClientProvider } from "@tanstack/react-query"
import { MemoryRouter, Route, Routes } from "react-router"
import TaskList from "~/pages/TaskList"

export function TaskListStoryFrame() {
  const [queryClient] = useState(() => new QueryClient({ defaultOptions: { queries: { retry: false } } }))
  return (
    <QueryClientProvider client={queryClient}>
      <MemoryRouter initialEntries={["/tasks"]}>
        <Routes><Route path="/tasks" element={<TaskList />} /></Routes>
      </MemoryRouter>
    </QueryClientProvider>
  )
}
