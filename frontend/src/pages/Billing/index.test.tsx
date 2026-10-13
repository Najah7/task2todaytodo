import { afterEach, expect, test, vi } from "vitest"
import { cleanup, render, screen, within } from "@testing-library/react"
import { QueryClient, QueryClientProvider } from "@tanstack/react-query"
import { createMemoryRouter, RouterProvider } from "react-router"
import { LanguageProviderContext } from "~/features/i18n/languageContext"
import BillingPage from "."
import { clearPendingTeamInvitation, pendingTeamInvitation, rememberTeamInvitation } from "~/features/project/lib/teamInvitationReturn"

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

test("shows the effective access plan when a paid subscription is not active for entitlements", async () => {
  vi.stubGlobal("fetch", vi.fn(async () => new Response(JSON.stringify({
    plan: "team", access_plan: "free", status: "past_due",
  }), { status: 200, headers: { "Content-Type": "application/json" } })))
  const client = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } })
  const router = createMemoryRouter([{ path: "/billing", element: <BillingPage /> }], { initialEntries: ["/billing"] })

  render(
    <LanguageProviderContext.Provider value={{ language: "ja", setLanguage: vi.fn() }}>
      <QueryClientProvider client={client}><RouterProvider router={router} /></QueryClientProvider>
    </LanguageProviderContext.Provider>,
  )

  const freeCard = (await screen.findByRole("heading", { name: "Free" })).closest("article")
  const teamCard = screen.getByRole("heading", { name: "Team" }).closest("article")
  expect(freeCard).toBeTruthy()
  expect(teamCard).toBeTruthy()
  expect(within(freeCard as HTMLElement).getByText("現在のプラン")).toBeTruthy()
  expect(within(teamCard as HTMLElement).queryByText("現在のプラン")).toBeNull()
  expect(screen.getByRole("button", { name: "Stripeポータルを開く" })).toBeTruthy()
})

test("returns to the same pending invitation when Team access is active", async () => {
  rememberTeamInvitation("invite-return-token")
  vi.stubGlobal("fetch", vi.fn(async () => new Response(JSON.stringify({
    plan: "team", access_plan: "team", status: "active",
  }), { status: 200, headers: { "Content-Type": "application/json" } })))
  const client = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } })
  const router = createMemoryRouter([
    { path: "/billing", element: <BillingPage /> },
    { path: "/invitations", element: <p>Invitation restored</p> },
  ], { initialEntries: ["/billing?checkout=success"] })

  render(
    <LanguageProviderContext.Provider value={{ language: "ja", setLanguage: vi.fn() }}>
      <QueryClientProvider client={client}><RouterProvider router={router} /></QueryClientProvider>
    </LanguageProviderContext.Provider>,
  )

  expect(await screen.findByText("Invitation restored")).toBeTruthy()
  expect(router.state.location.pathname).toBe("/invitations")
  expect(router.state.location.hash).toBe("#token=invite-return-token")
  expect(pendingTeamInvitation()).toBeNull()
  clearPendingTeamInvitation()
})
