import { afterEach, expect, test, vi } from "vitest"
import { cleanup, render, screen } from "@testing-library/react"
import { QueryClient, QueryClientProvider } from "@tanstack/react-query"
import { createMemoryRouter, RouterProvider } from "react-router"
import { LanguageProviderContext } from "~/features/i18n/languageContext"
import BillingPage from "."

afterEach(() => {
  cleanup()
  vi.unstubAllGlobals()
})

test("incomplete subscription is shown as payment pending and blocks duplicate checkout", async () => {
  const fetchMock = vi.fn(async () => new Response(JSON.stringify({ plan: "pro", status: "incomplete" }), {
    status: 200,
    headers: { "Content-Type": "application/json" },
  }))
  vi.stubGlobal("fetch", fetchMock)
  const client = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } })
  const router = createMemoryRouter([{ path: "/billing", element: <BillingPage /> }], { initialEntries: ["/billing"] })

  render(
    <LanguageProviderContext.Provider value={{ language: "ja", setLanguage: vi.fn() }}>
      <QueryClientProvider client={client}><RouterProvider router={router} /></QueryClientProvider>
    </LanguageProviderContext.Provider>,
  )

  expect(await screen.findByText("お支払い待ち")).toBeTruthy()
  expect(screen.getByRole("button", { name: "Stripeポータルを開く" })).toBeTruthy()
  expect(screen.queryByRole("button", { name: "プランを選択" })).toBeNull()
  expect(fetchMock).toHaveBeenCalledTimes(1)
})
