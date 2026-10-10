import { afterEach, expect, test, vi } from "vitest"
import { cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react"
import { QueryClient, QueryClientProvider } from "@tanstack/react-query"
import { createMemoryRouter, RouterProvider } from "react-router"
import type { RestProjectOptionsResponse, RestProjectResponse } from "~/api/generated/projects"
import { getGetProjectsIdQueryKey } from "~/api/generated/projects"
import { LanguageProviderContext } from "~/features/i18n/languageContext"
import ProjectsEditPage from "."

afterEach(() => {
  cleanup()
  vi.unstubAllGlobals()
})

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

const options: RestProjectOptionsResponse = {
  types: [{ label: "Other", label_jp: "その他", value: "other" }],
  priorities: [{ label: "Low", label_jp: "低", value: "low", weight: 1 }],
  statuses: [],
}

function jsonResponse(value: unknown, status = 200) {
  return new Response(JSON.stringify(value), { status, headers: { "Content-Type": "application/json" } })
}

test("keeps project draft and cache when explicit reload returns a non-editable project", async () => {
  let resolveReload!: (response: Response) => void
  const reloadResponse = new Promise<Response>((resolve) => { resolveReload = resolve })
  const fetchMock = vi.fn(async (input: RequestInfo | URL, _init?: RequestInit): Promise<Response> => {
    const url = String(input)
    if (url.endsWith("/api/projects/project-1") && _init?.method === "PATCH") {
      return jsonResponse({ error: { code: "revision_conflict", details: [{ code: "revision_conflict" }] } }, 409)
    }
    if (url.endsWith("/api/projects/project-1")) {
      if (fetchMock.mock.calls.filter(([request, init]) => String(request).endsWith("/api/projects/project-1") && init?.method !== "PATCH").length > 1) {
        return reloadResponse
      }
      return jsonResponse(project)
    }
    if (url.endsWith("/api/projects/options")) return jsonResponse(options)
    throw new Error(`Unexpected request: ${url} ${_init?.method ?? "GET"}`)
  })
  vi.stubGlobal("fetch", fetchMock)

  const client = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } })
  const router = createMemoryRouter([{ path: "/projects/:id/edit", element: <ProjectsEditPage /> }], { initialEntries: ["/projects/project-1/edit"] })
  render(
    <LanguageProviderContext.Provider value={{ language: "ja", setLanguage: vi.fn() }}>
      <QueryClientProvider client={client}><RouterProvider router={router} /></QueryClientProvider>
    </LanguageProviderContext.Provider>,
  )

  const title = await screen.findByLabelText(/プロジェクト名/)
  fireEvent.change(title, { target: { value: "Draft to preserve" } })
  fireEvent.click(screen.getByRole("button", { name: /保存する/ }))
  await screen.findByRole("button", { name: /最新の内容を読み込む/ })
  fireEvent.click(screen.getByRole("button", { name: /最新の内容を読み込む/ }))

  const dialog = await screen.findByRole("alertdialog", { name: /最新の内容を読み込みますか/ })
  const reloadButton = within(dialog).getByRole("button", { name: /最新の内容を読み込む/ })
  fireEvent.click(reloadButton)
  await waitFor(() => expect((within(dialog).getByRole("button", { name: /最新の内容を読み込む/ }) as HTMLButtonElement).disabled).toBe(true))
  resolveReload(jsonResponse({ ...project, can_update: false }, 200))

  await waitFor(() => expect((within(screen.getByRole("alertdialog")).getByRole("button", { name: /最新の内容を読み込む/ }) as HTMLButtonElement).disabled).toBe(false))
  expect((screen.getByLabelText(/プロジェクト名/) as HTMLInputElement).value).toBe("Draft to preserve")
  expect(screen.getByRole("alertdialog")).toBeTruthy()
  expect(client.getQueryData(getGetProjectsIdQueryKey("project-1"))).toEqual(project)
})

test("waits for initial project data before mounting fields, then hides loading state when ready", async () => {
  let resolveProject!: (response: Response) => void
  const projectResponse = new Promise<Response>((resolve) => { resolveProject = resolve })
  const fetchMock = vi.fn(async (input: RequestInfo | URL): Promise<Response> => {
    const url = String(input)
    if (url.endsWith("/api/projects/project-1")) return projectResponse
    if (url.endsWith("/api/projects/options")) return jsonResponse(options)
    throw new Error(`Unexpected request: ${url}`)
  })
  vi.stubGlobal("fetch", fetchMock)

  const client = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } })
  const router = createMemoryRouter([{ path: "/projects/:id/edit", element: <ProjectsEditPage /> }], { initialEntries: ["/projects/project-1/edit"] })
  render(
    <LanguageProviderContext.Provider value={{ language: "ja", setLanguage: vi.fn() }}>
      <QueryClientProvider client={client}><RouterProvider router={router} /></QueryClientProvider>
    </LanguageProviderContext.Provider>,
  )

  expect(screen.getByRole("status")).toBeTruthy()
  expect(screen.queryByLabelText(/プロジェクト名/)).toBeNull()
  resolveProject(jsonResponse(project))

  expect((await screen.findByLabelText(/プロジェクト名/) as HTMLInputElement).value).toBe("Original project")
  expect(screen.queryByRole("status")).toBeNull()
})

test("preserves an edit draft after a background project refetch", async () => {
  let projectRequests = 0
  const fetchMock = vi.fn(async (input: RequestInfo | URL): Promise<Response> => {
    const url = String(input)
    if (url.endsWith("/api/projects/project-1")) {
      projectRequests += 1
      return jsonResponse(projectRequests === 1 ? project : { ...project, title: "Background update" })
    }
    if (url.endsWith("/api/projects/options")) return jsonResponse(options)
    throw new Error(`Unexpected request: ${url}`)
  })
  vi.stubGlobal("fetch", fetchMock)

  const client = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } })
  const router = createMemoryRouter([{ path: "/projects/:id/edit", element: <ProjectsEditPage /> }], { initialEntries: ["/projects/project-1/edit"] })
  render(
    <LanguageProviderContext.Provider value={{ language: "ja", setLanguage: vi.fn() }}>
      <QueryClientProvider client={client}><RouterProvider router={router} /></QueryClientProvider>
    </LanguageProviderContext.Provider>,
  )

  const title = await screen.findByLabelText(/プロジェクト名/)
  fireEvent.change(title, { target: { value: "Draft survives refresh" } })
  await client.invalidateQueries({ queryKey: getGetProjectsIdQueryKey("project-1") })

  await waitFor(() => expect(client.getQueryData<RestProjectResponse>(getGetProjectsIdQueryKey("project-1"))?.title).toBe("Background update"))
  expect((screen.getByLabelText(/プロジェクト名/) as HTMLInputElement).value).toBe("Draft survives refresh")
  expect(screen.queryByRole("status")).toBeNull()
})

test("resets draft and revision when route changes to a different project", async () => {
  const secondProject = { ...project, id: "project-2", title: "Second project", revision: 8 }
  let updateCall: [RequestInfo | URL, RequestInit | undefined] | undefined
  const fetchMock = vi.fn(async (input: RequestInfo | URL, init?: RequestInit): Promise<Response> => {
    const url = String(input)
    if (url.endsWith("/api/projects/options")) return jsonResponse(options)
    if (url.endsWith("/api/projects/project-1")) return jsonResponse(project)
    if (url.endsWith("/api/projects/project-2") && init?.method === "PATCH") {
      updateCall = [input, init]
      return jsonResponse({ ...secondProject, title: "Updated second project", revision: 9 })
    }
    if (url.endsWith("/api/projects/project-2")) return jsonResponse(secondProject)
    throw new Error(`Unexpected request: ${url} ${init?.method ?? "GET"}`)
  })
  vi.stubGlobal("fetch", fetchMock)

  const client = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } })
  const router = createMemoryRouter([{ path: "/projects/:id/edit", element: <ProjectsEditPage /> }], { initialEntries: ["/projects/project-1/edit"] })
  render(
    <LanguageProviderContext.Provider value={{ language: "ja", setLanguage: vi.fn() }}>
      <QueryClientProvider client={client}><RouterProvider router={router} /></QueryClientProvider>
    </LanguageProviderContext.Provider>,
  )

  fireEvent.change(await screen.findByLabelText(/プロジェクト名/), { target: { value: "First project draft" } })
  await router.navigate("/projects/project-2/edit")
  const discardDialog = await screen.findByRole("alertdialog", { name: /変更を破棄/ })
  fireEvent.click(within(discardDialog).getByRole("button", { name: /破棄して移動|Discard and leave/ }))

  await waitFor(() => expect(router.state.location.pathname).toBe("/projects/project-2/edit"))
  const title = await screen.findByDisplayValue("Second project")
  expect((title as HTMLInputElement).value).toBe("Second project")
  fireEvent.change(title, { target: { value: "Updated second project" } })
  fireEvent.click(screen.getByRole("button", { name: /保存する/ }))

  await waitFor(() => expect(updateCall).toBeDefined())
  expect(String(updateCall?.[0])).toContain("/api/projects/project-2")
  expect(new Headers(updateCall?.[1]?.headers).get("If-Match")).toBe('"8"')
})

test("uses latest project revision when options load finishes after a project refetch", async () => {
  let resolveOptions!: (response: Response) => void
  const optionsResponse = new Promise<Response>((resolve) => { resolveOptions = resolve })
  let projectRequests = 0
  let updateCall: [RequestInfo | URL, RequestInit | undefined] | undefined
  const fetchMock = vi.fn(async (input: RequestInfo | URL, init?: RequestInit): Promise<Response> => {
    const url = String(input)
    if (url.endsWith("/api/projects/options")) return optionsResponse
    if (url.endsWith("/api/projects/project-1") && init?.method === "PATCH") {
      updateCall = [input, init]
      return jsonResponse({ ...project, revision: 5 })
    }
    if (url.endsWith("/api/projects/project-1")) {
      projectRequests += 1
      return jsonResponse(projectRequests === 1 ? project : { ...project, title: "Latest while options load", revision: 4 })
    }
    throw new Error(`Unexpected request: ${url} ${init?.method ?? "GET"}`)
  })
  vi.stubGlobal("fetch", fetchMock)

  const client = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } })
  const router = createMemoryRouter([{ path: "/projects/:id/edit", element: <ProjectsEditPage /> }], { initialEntries: ["/projects/project-1/edit"] })
  render(
    <LanguageProviderContext.Provider value={{ language: "ja", setLanguage: vi.fn() }}>
      <QueryClientProvider client={client}><RouterProvider router={router} /></QueryClientProvider>
    </LanguageProviderContext.Provider>,
  )

  await waitFor(() => expect(fetchMock.mock.calls.filter(([input]) => String(input).endsWith("/api/projects/project-1")).length).toBe(1))
  await waitFor(() => expect(fetchMock.mock.calls.some(([input]) => String(input).endsWith("/api/projects/options"))).toBe(true))
  await client.invalidateQueries({ queryKey: getGetProjectsIdQueryKey("project-1") })
  await waitFor(() => expect(client.getQueryData<RestProjectResponse>(getGetProjectsIdQueryKey("project-1"))?.revision).toBe(4))
  resolveOptions(jsonResponse(options))

  await screen.findByDisplayValue("Latest while options load")
  expect(screen.queryByRole("status")).toBeNull()
  fireEvent.click(screen.getByRole("button", { name: /保存する/ }))
  await waitFor(() => expect(updateCall).toBeDefined())
  expect(new Headers(updateCall?.[1]?.headers).get("If-Match")).toBe('"4"')
})

test("shows forbidden state without form or retry action after an initial 403", async () => {
  const fetchMock = vi.fn(async (input: RequestInfo | URL): Promise<Response> => {
    const url = String(input)
    if (url.endsWith("/api/projects/project-1")) return jsonResponse({ error: { code: "forbidden" } }, 403)
    throw new Error(`Unexpected request: ${url}`)
  })
  vi.stubGlobal("fetch", fetchMock)

  const client = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } })
  const router = createMemoryRouter([{ path: "/projects/:id/edit", element: <ProjectsEditPage /> }], { initialEntries: ["/projects/project-1/edit"] })
  render(
    <LanguageProviderContext.Provider value={{ language: "ja", setLanguage: vi.fn() }}>
      <QueryClientProvider client={client}><RouterProvider router={router} /></QueryClientProvider>
    </LanguageProviderContext.Provider>,
  )

  expect(await screen.findByText("このプロジェクトを操作する権限がありません。")).toBeTruthy()
  expect(screen.queryByLabelText(/プロジェクト名/)).toBeNull()
  expect(screen.queryByRole("button", { name: /再試行|Retry|再読み込み|Reload/ })).toBeNull()
  expect(fetchMock.mock.calls.some(([input]) => String(input).endsWith("/api/projects/options"))).toBe(false)
})
