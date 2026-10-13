import { afterEach, expect, test, vi } from "vitest"
import { cleanup, fireEvent, render, screen } from "@testing-library/react"
import { QueryClient, QueryClientProvider } from "@tanstack/react-query"
import { createMemoryRouter, RouterProvider } from "react-router"
import { LanguageProviderContext } from "~/features/i18n/languageContext"
import { pendingTeamInvitation } from "~/features/project/lib/teamInvitationReturn"
import BillingPage from "~/pages/Billing"
import InvitationAcceptancePage from "."

afterEach(() => {
  cleanup()
  localStorage.clear()
  sessionStorage.clear()
  vi.unstubAllGlobals()
})

test("registers first, preserves the invitation, and routes writer acceptance to Team billing", async () => {
  const fetchMock = vi.fn(async (input: RequestInfo | URL) => {
    const url = new URL(typeof input === "string" ? input : input instanceof URL ? input.href : input.url, "http://localhost")
    if (url.pathname === "/api/invitations/preview") {
      return new Response(JSON.stringify({
        project_id: "project-1", project_title: "Shared project", email: "invitee@example.com",
        role_id: "editor", recipient_status: "new", status: "pending",
      }), { status: 200, headers: { "Content-Type": "application/json" } })
    }
    if (url.pathname === "/api/invitations/accept") {
      return new Response(JSON.stringify({ personal_access_token: "session-token", requires_team_subscription: true }), {
        status: 200, headers: { "Content-Type": "application/json" },
      })
    }
    if (url.pathname === "/api/billing") {
      return new Response(JSON.stringify({ plan: "free", access_plan: "free", status: "none" }), {
        status: 200, headers: { "Content-Type": "application/json" },
      })
    }
    throw new Error(`Unexpected request ${url.pathname}`)
  })
  vi.stubGlobal("fetch", fetchMock)
  const client = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } })
  const router = createMemoryRouter([
    { path: "/invitations", element: <InvitationAcceptancePage /> },
    { path: "/billing", element: <BillingPage /> },
  ], { initialEntries: ["/invitations#token=invitation-secret"] })

  render(
    <LanguageProviderContext.Provider value={{ language: "ja", setLanguage: vi.fn() }}>
      <QueryClientProvider client={client}><RouterProvider router={router} /></QueryClientProvider>
    </LanguageProviderContext.Provider>,
  )

  fireEvent.change(await screen.findByLabelText("パスワード"), { target: { value: "Password1!" } })
  fireEvent.change(screen.getByLabelText("パスワード（確認）"), { target: { value: "Password1!" } })
  fireEvent.click(screen.getByRole("button", { name: "招待を受け入れる" }))

  expect(await screen.findByText("Teamプランに登録すると、この招待に戻って受け入れを完了できます。")).toBeTruthy()
  expect(router.state.location.pathname).toBe("/billing")
  expect(router.state.location.search).toBe("?plan=team")
  expect(pendingTeamInvitation()).toBe("invitation-secret")
  expect(localStorage.getItem("personal_access_token")).toBe("session-token")
})
