import { afterEach, expect, test, vi } from "vitest"
import { cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react"
import { QueryClient, QueryClientProvider } from "@tanstack/react-query"
import { createMemoryRouter, RouterProvider } from "react-router"
import type { RestProjectListResponse, RestProjectOptionsResponse, RestProjectResponse } from "~/api/generated/projects"
import { LanguageProviderContext } from "~/features/i18n/languageContext"
import { NotificationViewport } from "~/features/shared/notification"
import ProjectList from "."

afterEach(() => {
  cleanup()
  vi.unstubAllGlobals()
})

const project: RestProjectResponse = {
  can_delete: true,
  can_update: true,
  created_at: 1,
  deleted_at: null,
  description: "",
  end_date: "2026-10-20",
  goal: "A project",
  id: "project-1",
  priority: { label: "High", label_jp: "高", value: "high", weight: 3 },
  progress: 25,
  remaining_days: 11,
  revision: 4,
  start_date: null,
  status: "in_progress",
  title: "Conflict project",
  type: { label: "Other", label_jp: "その他", value: "other" },
  updated_at: 1,
  user_id: "user-1",
}

const list: RestProjectListResponse = {
  items: [project],
  next_page_token: "next-page",
  previous_page_token: "previous-page",
  summary: {
    due_soon_count: 1,
    overdue_count: 0,
    status_counts: { done: 0, in_progress: 1, open: 0, pending: 0, waiting_on_others: 0 },
    timezone: "Asia/Tokyo",
    today: "2026-10-09",
    total_count: 1,
    trash_count: 0,
  },
}

const options: RestProjectOptionsResponse = {
  types: [{ label: "Other", label_jp: "その他", value: "other" }],
  priorities: [{ label: "Low", label_jp: "低", value: "low", weight: 1 }],
  statuses: [
    { label: "In progress", label_jp: "進行中", value: "in_progress" },
    { label: "Pending", label_jp: "保留", value: "pending" },
    { label: "Done", label_jp: "完了", value: "done" },
    { label: "Open", label_jp: "オープン", value: "open" },
    { label: "Waiting on others", label_jp: "他者待ち", value: "waiting_on_others" },
  ],
}

function response(value: unknown, status = 200) {
  return new Response(value === undefined ? undefined : JSON.stringify(value), {
    status,
    headers: { "Content-Type": "application/json" },
  })
}

test("rolls back an optimistic status change after a revision conflict and refetches the list", async () => {
  let resolveStatus!: (response: Response) => void
  let markStatusStarted!: () => void
  const statusResponse = new Promise<Response>((resolve) => { resolveStatus = resolve })
  const statusStarted = new Promise<void>((resolve) => { markStatusStarted = resolve })
  const fetchMock = vi.fn(async (input: RequestInfo | URL, _init?: RequestInit): Promise<Response> => {
    const url = String(input)
    if (url.includes("/api/projects/options")) return response(options)
    if (url.includes("/api/projects/project-1/status")) {
      markStatusStarted()
      return statusResponse
    }
    if (url.includes("/api/projects")) return response(list)
    throw new Error(`Unexpected request: ${url}`)
  })
  vi.stubGlobal("fetch", fetchMock)

  const client = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } })
  const router = createMemoryRouter([{ path: "/projects", element: <ProjectList /> }], { initialEntries: ["/projects"] })
  render(
    <LanguageProviderContext.Provider value={{ language: "ja", setLanguage: vi.fn() }}>
      <QueryClientProvider client={client}>
        <RouterProvider router={router} />
        <NotificationViewport />
      </QueryClientProvider>
    </LanguageProviderContext.Provider>,
  )

  const status = await screen.findByRole("button", { name: "Conflict projectの状態 進行中" })
  const nextPage = await screen.findByRole("button", { name: "次へ" })
  await waitFor(() => expect((status as HTMLButtonElement).disabled).toBe(false))
  expect((nextPage as HTMLButtonElement).disabled).toBe(false)
  fireEvent.click(status)
  fireEvent.click(screen.getByRole("option", { name: "保留" }))

  await statusStarted
  const tabs = screen.getByRole("navigation", { name: "プロジェクトの状態" })
  await waitFor(() => expect(within(tabs).getAllByRole("button")[0]?.textContent).toContain("0"))
  expect((screen.getByRole("button", { name: "次へ" }) as HTMLButtonElement).disabled).toBe(true)

  resolveStatus(response({ error: { code: "revision_conflict", details: [{ field: "If-Match", code: "revision_conflict" }] } }, 409))
  await waitFor(() => expect(screen.getByRole("button", { name: "Conflict projectの状態 進行中" })).toBeTruthy())
  await waitFor(() => expect((screen.getByRole("button", { name: "次へ" }) as HTMLButtonElement).disabled).toBe(false))
  await waitFor(() => expect(fetchMock.mock.calls.filter(([input]) => String(input).includes("/api/projects?")).length).toBeGreaterThanOrEqual(2))
  const statusCall = fetchMock.mock.calls.find(([input]) => String(input).includes("/api/projects/project-1/status"))
  expect(statusCall).toBeTruthy()
  expect(new Headers(statusCall?.[1]?.headers).get("If-Match")).toBe('"4"')
  expect(screen.getAllByText("プロジェクトが更新されています。最新の内容を確認してください。")).toHaveLength(1)
})

test("replaces a stale pagination URL when returning to the first page", async () => {
  const fetchMock = vi.fn(async (input: RequestInfo | URL): Promise<Response> => {
    const url = new URL(String(input), window.location.origin)
    if (url.pathname === "/api/projects" && url.searchParams.get("page_token") === "stale") {
      return response({ ...list, items: [], next_page_token: "", previous_page_token: "" })
    }
    if (url.pathname === "/api/projects") return response(list)
    throw new Error(`Unexpected request: ${url}`)
  })
  vi.stubGlobal("fetch", fetchMock)

  const client = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } })
  const router = createMemoryRouter(
    [{ path: "/projects", element: <ProjectList /> }],
    {
      initialEntries: ["/projects?status=pending", "/projects?status=in_progress&page_token=stale"],
      initialIndex: 1,
    },
  )
  render(
    <LanguageProviderContext.Provider value={{ language: "ja", setLanguage: vi.fn() }}>
      <QueryClientProvider client={client}><RouterProvider router={router} /></QueryClientProvider>
    </LanguageProviderContext.Provider>,
  )

  await waitFor(() => expect(new URLSearchParams(router.state.location.search).get("page_token")).toBeNull())
  expect(new URLSearchParams(router.state.location.search).get("status")).toBe("in_progress")
  await router.navigate(-1)
  expect(new URLSearchParams(router.state.location.search).get("status")).toBe("pending")
  expect(fetchMock.mock.calls.some(([input]) => new URL(String(input), window.location.origin).searchParams.get("page_token") === "stale")).toBe(true)
})
