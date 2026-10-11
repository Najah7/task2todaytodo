import { useState } from "react"
import { QueryClient, QueryClientProvider } from "@tanstack/react-query"
import { MemoryRouter, Route, Routes } from "react-router"
import InboxPage from "~/pages/Inbox"

export function InboxStoryFrame() {
  const [queryClient] = useState(() => new QueryClient({ defaultOptions: { queries: { retry: false } } }))
  return <QueryClientProvider client={queryClient}>
    <MemoryRouter initialEntries={["/inbox"]}>
      <Routes><Route path="/inbox" element={<InboxPage />} /></Routes>
    </MemoryRouter>
  </QueryClientProvider>
}
