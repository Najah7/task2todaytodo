import { afterEach, expect, test, vi } from "vitest"
import { cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react"
import { QueryClient, QueryClientProvider } from "@tanstack/react-query"
import { createMemoryRouter, RouterProvider } from "react-router"
import { LanguageProviderContext } from "~/features/i18n/languageContext"
import { NotificationViewport } from "~/features/shared/notification"
import TaskList from "."

afterEach(() => {
  cleanup()
  vi.unstubAllGlobals()
})

function response(value: unknown, status = 200) {
  return new Response(JSON.stringify(value), {
    status,
    headers: { "Content-Type": "application/json" },
  })
}

const list = {
  action_item_completed_count: 0,
  action_item_total_count: 1,
  estimated_minutes_total: 30,
  items: [{
    action_item_completed_count: 0,
    action_item_count: 1,
    actual_minutes: null,
    assignee_id: "user-1",
    can_update: true,
    created_at: "2026-10-01T00:00:00Z",
    description: "",
    due_date: null,
    estimate_source: "action_items",
    estimated_minutes: 30,
    id: "task-1",
    manual_estimated_minutes: null,
    priority: "medium",
    progress: 0,
    project_id: null,
    project_name: null,
    remaining_days: null,
    revision: 1,
    status: "open",
    title: "Prepare the release",
    updated_at: "2026-10-01T00:00:00Z",
    user_id: "user-1",
  }],
  next_page_token: "",
  previous_page_token: "",
  status_counts: { open: 1, in_progress: 0, pending: 0, waiting_on_others: 0, done: 0 },
  total_count: 1,
}

const activeItem = {
  id: "item-1",
  task_id: "task-1",
  title: "Review the release notes",
  estimated_minutes: 30,
  completed: false,
}

test("completes an expanded action item, keeps its row busy, and preserves server state after a failed reopen", async () => {
  let resolveCompletion!: (result: Response) => void
  let markCompletionStarted!: () => void
  const completionResponse = new Promise<Response>((resolve) => { resolveCompletion = resolve })
  const completionStarted = new Promise<void>((resolve) => { markCompletionStarted = resolve })
  let actionItemsRead = 0
  let projectsRead = 0
  const fetchMock = vi.fn(async (input: RequestInfo | URL): Promise<Response> => {
    const url = new URL(String(input), window.location.origin)
    if (url.pathname === "/api/tasks") return response(list)
    if (url.pathname === "/api/projects") {
      projectsRead += 1
      return projectsRead === 1
        ? response({ items: [], next_page_token: "" })
        : response({ error: { message: "background project refresh failed" } }, 500)
    }
    if (url.pathname === "/api/tasks/task-1/action-items") {
      actionItemsRead += 1
      if (actionItemsRead > 2) return response({ error: { message: "action item refresh failed" } }, 500)
      return response({ items: [{ ...activeItem, completed: actionItemsRead > 1 }] })
    }
    if (url.pathname === "/api/tasks/task-1/action-items/item-1:complete") {
      markCompletionStarted()
      return completionResponse
    }
    if (url.pathname === "/api/tasks/task-1/action-items/item-1:reopen") return response({ error: { message: "reopen failed" } }, 500)
    throw new Error(`Unexpected request: ${url}`)
  })
  vi.stubGlobal("fetch", fetchMock)

  const client = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } })
  const router = createMemoryRouter([{ path: "/tasks", element: <TaskList /> }], { initialEntries: ["/tasks"] })
  render(
    <LanguageProviderContext.Provider value={{ language: "en", setLanguage: vi.fn() }}>
      <QueryClientProvider client={client}>
        <RouterProvider router={router} />
        <NotificationViewport />
      </QueryClientProvider>
    </LanguageProviderContext.Provider>,
  )

  fireEvent.click(await screen.findByRole("button", { name: "Show action items for Prepare the release" }))
  await screen.findByText("Review the release notes")
  const complete = screen.getByRole("button", { name: "Mark action item complete" })
  fireEvent.click(complete)

  await completionStarted
  await waitFor(() => expect((screen.getByRole("button", { name: "Mark action item complete" }) as HTMLButtonElement).disabled).toBe(true))
  resolveCompletion(response({ message: "completed" }))

  await screen.findByRole("button", { name: "Reopen action item" })
  await waitFor(() => expect((screen.getByRole("button", { name: "Add task" }) as HTMLButtonElement).disabled).toBe(false))
  expect(screen.queryByRole("alert")).toBeNull()
  expect(actionItemsRead).toBeGreaterThan(1)

  fireEvent.click(screen.getByRole("button", { name: "Reopen action item" }))
  await screen.findByText("Could not save the task. Check the submitted values.")
  await waitFor(() => expect(actionItemsRead).toBeGreaterThan(2))
  await waitFor(() => expect(screen.getByRole("button", { name: "Reopen action item" })).toBeTruthy())
  expect(screen.queryByText("Could not load action items")).toBeNull()
  expect(actionItemsRead).toBeGreaterThan(2)
})

test("keeps filters in the URL while paging and preserves explicit navigation history", async () => {
  const requestedUrls: URL[] = []
  const fetchMock = vi.fn(async (input: RequestInfo | URL): Promise<Response> => {
    const url = new URL(String(input), window.location.origin)
    if (url.pathname === "/api/tasks") {
      requestedUrls.push(url)
      return response({ ...list, next_page_token: "next-token", previous_page_token: "previous-token" })
    }
    if (url.pathname === "/api/projects") return response({ items: [], next_page_token: "" })
    throw new Error(`Unexpected request: ${url}`)
  })
  vi.stubGlobal("fetch", fetchMock)

  const client = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } })
  const router = createMemoryRouter(
    [{ path: "/tasks", element: <TaskList /> }],
    { initialEntries: ["/tasks?status=pending&title=older", "/tasks?status=in_progress&title=release"], initialIndex: 1 },
  )
  render(
    <LanguageProviderContext.Provider value={{ language: "en", setLanguage: vi.fn() }}>
      <QueryClientProvider client={client}><RouterProvider router={router} /></QueryClientProvider>
    </LanguageProviderContext.Provider>,
  )

  const next = await screen.findByRole("button", { name: "Next" })
  await waitFor(() => expect((next as HTMLButtonElement).disabled).toBe(false))
  fireEvent.click(next)
  await waitFor(() => expect(new URLSearchParams(router.state.location.search).get("page_token")).toBe("next-token"))
  expect(new URLSearchParams(router.state.location.search).get("status")).toBe("in_progress")
  expect(new URLSearchParams(router.state.location.search).get("title")).toBe("release")

  const tabs = within(screen.getByRole("navigation", { name: "Task status" }))
  fireEvent.click(tabs.getByRole("button", { name: /Done/ }))
  await waitFor(() => expect(new URLSearchParams(router.state.location.search).get("status")).toBe("done"))
  expect(new URLSearchParams(router.state.location.search).get("page_token")).toBeNull()
  expect(new URLSearchParams(router.state.location.search).get("title")).toBe("release")

  await router.navigate(-1)
  expect(new URLSearchParams(router.state.location.search).get("page_token")).toBe("next-token")
  expect(new URLSearchParams(router.state.location.search).get("status")).toBe("in_progress")
  expect(requestedUrls.some((url) => url.searchParams.get("title") === "release" && url.searchParams.get("page_token") === "next-token")).toBe(true)
})

test("offers explicit recovery from an empty page token and keeps it in navigation history", async () => {
  const fetchMock = vi.fn(async (input: RequestInfo | URL): Promise<Response> => {
    const url = new URL(String(input), window.location.origin)
    if (url.pathname === "/api/tasks") {
      return url.searchParams.get("page_token") === "stale"
        ? response({ ...list, items: [], next_page_token: "", previous_page_token: "", total_count: 0 })
        : response(list)
    }
    if (url.pathname === "/api/projects") return response({ items: [], next_page_token: "" })
    throw new Error(`Unexpected request: ${url}`)
  })
  vi.stubGlobal("fetch", fetchMock)

  const client = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } })
  const router = createMemoryRouter(
    [{ path: "/tasks", element: <TaskList /> }],
    { initialEntries: ["/tasks?status=pending", "/tasks?status=in_progress&page_token=stale"], initialIndex: 1 },
  )
  render(
    <LanguageProviderContext.Provider value={{ language: "en", setLanguage: vi.fn() }}>
      <QueryClientProvider client={client}><RouterProvider router={router} /></QueryClientProvider>
    </LanguageProviderContext.Provider>,
  )

  fireEvent.click(await screen.findByRole("button", { name: "Back to the first page" }))
  await waitFor(() => expect(new URLSearchParams(router.state.location.search).get("page_token")).toBeNull())
  expect(new URLSearchParams(router.state.location.search).get("status")).toBe("in_progress")
  expect(router.state.historyAction).toBe("PUSH")
  await router.navigate(-1)
  await waitFor(() => expect(new URLSearchParams(router.state.location.search).get("page_token")).toBe("stale"))
  await router.navigate(-1)
  await waitFor(() => expect(new URLSearchParams(router.state.location.search).get("status")).toBe("pending"))
})

test("retries unavailable project filters without requesting unrelated form options", async () => {
  let projectsRequests = 0
  const fetchMock = vi.fn(async (input: RequestInfo | URL): Promise<Response> => {
    const url = new URL(String(input), window.location.origin)
    if (url.pathname === "/api/tasks") return response(list)
    if (url.pathname === "/api/projects") {
      projectsRequests += 1
      return projectsRequests === 1
        ? response({ error: { message: "project options unavailable" } }, 500)
        : response({ items: [{ id: "project-1", title: "Alpha" }], next_page_token: "" })
    }
    throw new Error(`Unexpected request: ${url}`)
  })
  vi.stubGlobal("fetch", fetchMock)

  const client = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } })
  const router = createMemoryRouter([{ path: "/tasks", element: <TaskList /> }], { initialEntries: ["/tasks"] })
  render(
    <LanguageProviderContext.Provider value={{ language: "en", setLanguage: vi.fn() }}>
      <QueryClientProvider client={client}><RouterProvider router={router} /></QueryClientProvider>
    </LanguageProviderContext.Provider>,
  )

  await screen.findByRole("alert")
  const create = screen.getByRole("button", { name: "Add task" }) as HTMLButtonElement
  expect(create.disabled).toBe(true)
  fireEvent.click(screen.getByRole("button", { name: "Retry" }))

  await waitFor(() => expect(projectsRequests).toBe(2))
  await waitFor(() => expect((screen.getByRole("button", { name: "Add task" }) as HTMLButtonElement).disabled).toBe(false))
  expect(screen.queryByRole("alert")).toBeNull()
  expect(fetchMock.mock.calls.some(([input]) => String(input).includes("/api/projects/options"))).toBe(false)
})
