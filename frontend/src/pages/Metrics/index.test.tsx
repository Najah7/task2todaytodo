import { afterEach, expect, test, vi } from "vitest"
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react"
import { QueryClient, QueryClientProvider } from "@tanstack/react-query"
import { createMemoryRouter, RouterProvider } from "react-router"
import { LanguageProviderContext } from "~/features/i18n/languageContext"
import MetricsPage from "."

afterEach(() => {
  cleanup()
  vi.unstubAllGlobals()
})

function jsonResponse(value: unknown) {
  return new Response(JSON.stringify(value), { status: 200, headers: { "Content-Type": "application/json" } })
}

function completionResponse(plannedItems = 0) {
  return {
    timezone: "Asia/Tokyo",
    start_date: "2026-10-07",
    end_date: "2026-10-13",
    total_planned_items: plannedItems,
    completed_items: 0,
    completion_ratio: plannedItems > 0 ? 0 : null,
    days: plannedItems > 0
      ? [{ date: "2026-10-13", planned_items: plannedItems, completed_items: 0, completion_ratio: 0 }]
      : [],
  }
}

function renderMetrics(initialEntry = "/metrics") {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } })
  const router = createMemoryRouter([{ path: "/metrics", element: <MetricsPage /> }], { initialEntries: [initialEntry] })
  render(
    <LanguageProviderContext.Provider value={{ language: "ja", setLanguage: vi.fn() }}>
      <QueryClientProvider client={client}><RouterProvider router={router} /></QueryClientProvider>
    </LanguageProviderContext.Provider>,
  )
  return router
}

test("period switch updates URL and chart displays daily completion details", async () => {
  const fetchMock = vi.fn(async (input: RequestInfo | URL) => {
    const url = String(input)
    const pathname = new URL(url, "http://localhost").pathname
    if (pathname === "/api/metrics/todo-completion") return jsonResponse(completionResponse(1))
    if (pathname === "/api/metrics/overdue-work") return jsonResponse({ timezone: "Asia/Tokyo", as_of_date: "2026-10-13", total_overdue_count: 0, projects: [], other_count: 0 })
    throw new Error(`Unexpected request: ${url}`)
  })
  vi.stubGlobal("fetch", fetchMock)
  const router = renderMetrics()

  expect(await screen.findByRole("heading", { name: "メトリクス" })).toBeTruthy()
  expect(screen.getByRole("columnheader", { name: "完了率" })).toBeTruthy()
  expect(screen.getByRole("cell", { name: "0%" })).toBeTruthy()
  fireEvent.click(screen.getByRole("button", { name: "30日間" }))
  await waitFor(() => expect(router.state.location.search).toBe("?days=30"))
  await waitFor(() => expect(fetchMock.mock.calls.some(([input]) => String(input).includes("days=30"))).toBe(true))
  fireEvent.click(screen.getByRole("button", { name: "7日間" }))
  await waitFor(() => expect(router.state.location.search).toBe(""))
})

test("zero planned work shows no-data state instead of a zero completion rate", async () => {
  vi.stubGlobal("fetch", vi.fn(async (input: RequestInfo | URL) => {
    const pathname = new URL(String(input), "http://localhost").pathname
    if (pathname === "/api/metrics/todo-completion") return jsonResponse(completionResponse())
    if (pathname === "/api/metrics/overdue-work") return jsonResponse({ timezone: "Asia/Tokyo", as_of_date: "2026-10-13", total_overdue_count: 0, projects: [], other_count: 0 })
    throw new Error(`Unexpected request: ${String(input)}`)
  }))
  renderMetrics()

  expect(await screen.findByText("計画した項目はありません")).toBeTruthy()
  expect(screen.getByText("—")).toBeTruthy()
  expect(screen.getByText("この期間に表示できるデータはありません。")).toBeTruthy()
})

test("load failure exposes retry and renders data after retry", async () => {
  let fail = true
  vi.stubGlobal("fetch", vi.fn(async (input: RequestInfo | URL) => {
    if (fail) throw new Error("offline")
    const pathname = new URL(String(input), "http://localhost").pathname
    if (pathname === "/api/metrics/todo-completion") return jsonResponse(completionResponse())
    if (pathname === "/api/metrics/overdue-work") return jsonResponse({ timezone: "Asia/Tokyo", as_of_date: "2026-10-13", total_overdue_count: 0, projects: [], other_count: 0 })
    throw new Error(`Unexpected request: ${String(input)}`)
  }))
  renderMetrics()

  expect(await screen.findByText("メトリクスを読み込めませんでした。")).toBeTruthy()
  fail = false
  fireEvent.click(screen.getByRole("button", { name: "再試行" }))
  expect(await screen.findByText("期限を過ぎた項目はありません。")).toBeTruthy()
})
