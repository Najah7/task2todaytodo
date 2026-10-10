import { QueryClient, QueryClientProvider } from "@tanstack/react-query"
import { cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react"
import { createMemoryRouter, RouterProvider } from "react-router"
import { afterEach, expect, test, vi } from "vitest"
import type { RestProjectOptionsResponse, RestProjectResponse } from "~/api/generated/projects"
import { LanguageProviderContext } from "~/features/i18n/languageContext"
import ProjectsEditPage from "~/pages/ProjectsEdit"
import ProjectsNewPage from "~/pages/ProjectsNew"

afterEach(() => {
  cleanup()
  vi.unstubAllGlobals()
})

const options: RestProjectOptionsResponse = {
  types: [{ label: "Other", label_jp: "その他", value: "other" }],
  priorities: [{ label: "Low", label_jp: "低", value: "low", weight: 1 }],
  statuses: [],
}

const project: RestProjectResponse = {
  can_delete: true,
  can_update: true,
  created_at: 1,
  deleted_at: null,
  description: "Initial description",
  end_date: null,
  goal: "Initial goal",
  id: "project-1",
  priority: { label: "Low", label_jp: "低", value: "low", weight: 1 },
  progress: 0,
  remaining_days: null,
  revision: 3,
  start_date: null,
  status: "open",
  title: "Original project",
  type: { label: "Other", label_jp: "その他", value: "other" },
  updated_at: 1,
  user_id: "user-1",
}

function jsonResponse(value: unknown, status = 200) {
  return new Response(JSON.stringify(value), { status, headers: { "Content-Type": "application/json" } })
}

function renderForm({ mode = "create", fetch: fetchImpl }: { mode?: "create" | "edit"; fetch?: typeof fetch } = {}) {
  const fetchMock = fetchImpl ?? vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
    const url = String(input)
    if (url.endsWith("/api/projects/options")) return jsonResponse(options)
    if (url.endsWith("/api/projects/project-1") && init?.method === "PATCH") return jsonResponse({ ...project, revision: 4 })
    if (url.endsWith("/api/projects/project-1")) return jsonResponse(project)
    if (url.endsWith("/api/projects") && init?.method === "POST") return jsonResponse({ ...project, id: "project-created" })
    if (url.includes("/api/projects?")) return jsonResponse({
      items: [],
      next_page_token: "",
      previous_page_token: "",
      summary: {
        due_soon_count: 0,
        overdue_count: 0,
        status_counts: { done: 0, in_progress: 0, open: 0, pending: 0, waiting_on_others: 0 },
        timezone: "Asia/Tokyo",
        today: "2026-10-09",
        total_count: 0,
        trash_count: 0,
      },
    })
    throw new Error(`Unexpected request: ${url} ${init?.method ?? "GET"}`)
  })
  vi.stubGlobal("fetch", fetchMock)

  const client = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } })
  const router = createMemoryRouter([
    { path: "/projects", element: <h1>Projects open</h1> },
    { path: "/projects/new", element: <ProjectsNewPage /> },
    { path: "/projects/:id/edit", element: <ProjectsEditPage /> },
  ], { initialEntries: [mode === "edit" ? `/projects/${project.id}/edit` : "/projects/new"] })
  render(
    <LanguageProviderContext.Provider value={{ language: "ja", setLanguage: vi.fn() }}>
      <QueryClientProvider client={client}><RouterProvider router={router} /></QueryClientProvider>
    </LanguageProviderContext.Provider>,
  )
  return { client, fetchMock, router }
}

test("keeps fields unmounted through loading and error, then shows them after retry succeeds", async () => {
  let resolveFirstRequest: ((response: Response) => void) | undefined
  let optionRequests = 0
  const fetchMock = vi.fn(async (input: RequestInfo | URL) => {
    const url = String(input)
    if (url.endsWith("/api/projects/options")) {
      optionRequests += 1
      if (optionRequests === 1) {
        return await new Promise<Response>((resolve) => { resolveFirstRequest = resolve })
      }
      return jsonResponse(options)
    }
    throw new Error(`Unexpected request: ${url}`)
  })

  renderForm({ fetch: fetchMock })
  expect(screen.getByRole("status")).toBeTruthy()
  expect(screen.queryByLabelText(/プロジェクト名|Project name/)).toBeNull()

  await waitFor(() => expect(resolveFirstRequest).toBeTypeOf("function"))
  resolveFirstRequest!(jsonResponse({ error: { message: "unavailable" } }, 503))
  expect(await screen.findByText("プロジェクトの編集内容を読み込めませんでした。")).toBeTruthy()
  expect(screen.queryByLabelText(/プロジェクト名|Project name/)).toBeNull()

  fireEvent.click(screen.getByRole("button", { name: /再試行|Retry/ }))
  expect(await screen.findByLabelText(/プロジェクト名|Project name/)).toBeTruthy()
  expect(screen.queryByText("プロジェクトの編集内容を読み込めませんでした。")).toBeNull()
  expect(screen.queryByRole("status")).toBeNull()
})

test("shows required title and date order validation beside their fields", async () => {
  renderForm()
  const submit = await screen.findByRole("button", { name: /作成する|Create/ })
  fireEvent.click(submit)
  expect(await screen.findByText("プロジェクト名を入力してください。" )).toBeTruthy()

  fireEvent.change(screen.getByLabelText(/プロジェクト名|Project name/), { target: { value: "Quarterly report" } })
  fireEvent.change(screen.getByLabelText(/開始日|Start date/), { target: { value: "2026-10-20" } })
  fireEvent.change(screen.getByLabelText(/期限|Deadline/), { target: { value: "2026-10-10" } })
  fireEvent.click(submit)
  expect(await screen.findByText("期限は開始日以降の日付を入力してください。" )).toBeTruthy()
  expect(screen.getByLabelText(/期限|Deadline/).getAttribute("aria-invalid")).toBe("true")
})

test("preserves a dirty draft until the user confirms leaving", async () => {
  const { router } = renderForm()
  fireEvent.change(await screen.findByLabelText(/プロジェクト名|Project name/), { target: { value: "A draft" } })
  fireEvent.click(screen.getByRole("button", { name: /キャンセル|Cancel/ }))

  const dialog = await screen.findByRole("alertdialog", { name: /変更を破棄|Discard changes/ })
  fireEvent.click(within(dialog).getByRole("button", { name: /編集を続ける|Keep editing/ }))
  await waitFor(() => expect(screen.queryByRole("alertdialog")).toBeNull())
  expect((screen.getByLabelText(/プロジェクト名|Project name/) as HTMLInputElement).value).toBe("A draft")
  expect(router.state.location.pathname).toBe("/projects/new")
})

test("requires explicit discard confirmation before loading the current revision", async () => {
  let projectRequests = 0
  const latest = { ...project, title: "Latest" }
  const { fetchMock } = renderForm({
    mode: "edit",
    fetch: vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input)
      if (url.endsWith("/api/projects/options")) return jsonResponse(options)
      if (url.endsWith("/api/projects/project-1") && init?.method === "PATCH") return jsonResponse({ error: { message: "conflict", details: [{ field: "If-Match", code: "revision_conflict" }] } }, 409)
      if (url.endsWith("/api/projects/project-1")) return jsonResponse(++projectRequests === 1 ? project : latest)
      throw new Error(`Unexpected request: ${url} ${init?.method ?? "GET"}`)
    }),
  })
  fireEvent.change(await screen.findByLabelText(/プロジェクト名|Project name/), { target: { value: "My draft" } })
  fireEvent.click(screen.getByRole("button", { name: /保存する|Save/ }))
  await screen.findByRole("button", { name: /最新の内容を読み込む|Load latest content/ })
  fireEvent.click(screen.getByRole("button", { name: /最新の内容を読み込む|Load latest content/ }))
  const dialog = await screen.findByRole("alertdialog", { name: /最新の内容を読み込みますか|Load the latest content/ })
  expect((screen.getByLabelText(/プロジェクト名|Project name/) as HTMLInputElement).value).toBe("My draft")
  fireEvent.click(within(dialog).getByRole("button", { name: /最新の内容を読み込む|Load latest content/ }))
  await waitFor(() => expect((screen.getByLabelText(/プロジェクト名|Project name/) as HTMLInputElement).value).toBe("Latest"))
  expect(fetchMock).toHaveBeenCalled()
})

test("shows a server start-date error beside its input", async () => {
  renderForm({ fetch: vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
    const url = String(input)
    if (url.endsWith("/api/projects/options")) return jsonResponse(options)
    if (url.endsWith("/api/projects") && init?.method === "POST") return jsonResponse({ error: { details: [{ field: "start_date", code: "invalid_date" }] } }, 422)
    throw new Error(`Unexpected request: ${url} ${init?.method ?? "GET"}`)
  }) })
  fireEvent.change(await screen.findByLabelText(/プロジェクト名|Project name/), { target: { value: "A project" } })
  fireEvent.click(screen.getByRole("button", { name: /作成する|Create/ }))

  const startDate = screen.getByLabelText(/開始日|Start date/)
  expect(await screen.findByText(/有効な日付を入力してください|Enter a valid date/)).toBeTruthy()
  expect(startDate.getAttribute("aria-invalid")).toBe("true")
  expect(startDate.getAttribute("aria-describedby")).toBe("project-start-date-error")
})

test("keeps the draft and reload confirmation when loading latest revision fails", async () => {
  let projectRequests = 0
  renderForm({ mode: "edit", fetch: vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
    const url = String(input)
    if (url.endsWith("/api/projects/options")) return jsonResponse(options)
    if (url.endsWith("/api/projects/project-1") && init?.method === "PATCH") return jsonResponse({ error: { message: "conflict", details: [{ field: "If-Match", code: "revision_conflict" }] } }, 409)
    if (url.endsWith("/api/projects/project-1")) return jsonResponse(++projectRequests === 1 ? project : { error: { message: "unavailable" } }, projectRequests === 1 ? 200 : 503)
    throw new Error(`Unexpected request: ${url} ${init?.method ?? "GET"}`)
  }) })
  // A conflict is produced by an edit submission, then the explicit reload is failed.
  fireEvent.change(await screen.findByLabelText(/プロジェクト名|Project name/), { target: { value: "Keep this draft" } })
  fireEvent.click(screen.getByRole("button", { name: /保存する|Save/ }))
  await screen.findByRole("button", { name: /最新の内容を読み込む|Load latest content/ })
  fireEvent.click(screen.getByRole("button", { name: /最新の内容を読み込む|Load latest content/ }))
  const dialog = await screen.findByRole("alertdialog", { name: /最新の内容を読み込みますか|Load the latest content/ })
  fireEvent.click(within(dialog).getByRole("button", { name: /最新の内容を読み込む|Load latest content/ }))
  await waitFor(() => expect((screen.getByLabelText(/プロジェクト名|Project name/) as HTMLInputElement).value).toBe("Keep this draft"))
  expect(screen.getByRole("alertdialog")).toBeTruthy()
})

test("clears dirty draft before successful save navigation", async () => {
  const { fetchMock, router } = renderForm()
  fireEvent.change(await screen.findByLabelText(/プロジェクト名|Project name/), { target: { value: "A saved project" } })
  fireEvent.click(screen.getByRole("button", { name: /作成する|Create/ }))
  expect(await screen.findByRole("heading", { name: "Projects open" })).toBeTruthy()
  expect(router.state.location.pathname).toBe("/projects")
  expect(fetchMock).toHaveBeenCalled()
  expect(screen.queryByRole("alertdialog")).toBeNull()
})
