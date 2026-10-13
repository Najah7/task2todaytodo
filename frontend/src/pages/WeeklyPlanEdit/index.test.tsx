import { afterEach, expect, test, vi } from "vitest"
import { cleanup, render, screen } from "@testing-library/react"
import { QueryClient, QueryClientProvider } from "@tanstack/react-query"
import { createMemoryRouter, RouterProvider } from "react-router"
import { LanguageProviderContext } from "~/features/i18n/languageContext"
import WeeklyPlanEditPage from "."

afterEach(() => {
  cleanup()
  vi.unstubAllGlobals()
})

function jsonResponse(value: unknown) {
  return new Response(JSON.stringify(value), { status: 200, headers: { "Content-Type": "application/json" } })
}

test("waits for independent allocation resources before mounting the form", async () => {
  let resolvePlans!: (response: Response) => void
  const plansResponse = new Promise<Response>((resolve) => { resolvePlans = resolve })
  const fetchMock = vi.fn(async (input: RequestInfo | URL): Promise<Response> => {
    const url = String(input)
    const pathname = new URL(url, "http://localhost").pathname
    if (pathname === "/api/users/me") return jsonResponse({ user_id: "user-1", timezone: "Asia/Tokyo", active_start_time: "07:00", active_end_time: "23:00" })
    if (pathname === "/api/allocation-rules") return jsonResponse({ revision: "rules-v1", timezone: "Asia/Tokyo", items: [] })
    if (pathname === "/api/allocation-plans") return plansResponse
    if (pathname === "/api/projects") return jsonResponse({ items: [] })
    throw new Error(`Unexpected request: ${url}`)
  })
  vi.stubGlobal("fetch", fetchMock)

  const client = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } })
  const router = createMemoryRouter([{ path: "/weekly-plan/edit", element: <WeeklyPlanEditPage /> }], { initialEntries: ["/weekly-plan/edit?week_start=2026-10-12"] })
  render(
    <LanguageProviderContext.Provider value={{ language: "ja", setLanguage: vi.fn() }}>
      <QueryClientProvider client={client}><RouterProvider router={router} /></QueryClientProvider>
    </LanguageProviderContext.Provider>,
  )

  expect(await screen.findByText("週間プランを読み込んでいます…")).toBeTruthy()
  expect(screen.queryByLabelText("週の目標（分）")).toBeNull()
  resolvePlans(jsonResponse({ revision: "plans-v1", items: [] }))

  expect(await screen.findByText("追加できるプロジェクトはありません。")).toBeTruthy()
  expect(screen.queryByText("週間プランを読み込んでいます…")).toBeNull()
  expect(fetchMock.mock.calls.some(([input]) => new URL(String(input), "http://localhost").pathname.startsWith("/api/allocation-weeks/"))).toBe(false)
})
