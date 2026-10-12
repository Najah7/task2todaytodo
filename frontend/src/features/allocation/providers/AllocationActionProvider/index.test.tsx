import { afterEach, expect, test, vi } from "vitest"
import { useContext } from "react"
import { act, cleanup, fireEvent, render, screen } from "@testing-library/react"
import { QueryClient, QueryClientProvider } from "@tanstack/react-query"
import { LanguageProviderContext } from "~/features/i18n/languageContext"
import { AllocationAPIDataContext, type AllocationAPIData } from "~/features/allocation/providers/AllocationAPIDataProvider/context"
import { AllocationActionContext } from "./context"
import { AllocationActionProvider } from "."

afterEach(() => {
  cleanup()
  vi.unstubAllGlobals()
})

function response(value: unknown, status = 200) {
  return new Response(JSON.stringify(value), { status, headers: { "Content-Type": "application/json" } })
}

test("uses each ready snapshot token once, then reuses only confirmed resource revisions", async () => {
  const calls: Array<{ path: string; ifMatch: string | null }> = []
  const attempts = new Map<string, number>()
  vi.stubGlobal("fetch", vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
    const path = new URL(String(input), "http://localhost").pathname
    const headers = new Headers(init?.headers)
    calls.push({ path, ifMatch: headers.get("If-Match") })
    const count = (attempts.get(path) ?? 0) + 1
    attempts.set(path, count)
    if (path === "/api/allocation-rules") return response({ revision: count === 1 ? "rules-v2" : "rules-v3", timezone: "Asia/Tokyo", items: [] })
    if (path === "/api/allocation-plans" && count === 1) return response({ error: { message: "failed" } }, 500)
    if (path === "/api/allocation-plans") return response({ revision: "plans-v2", items: [] })
    throw new Error(`Unexpected request ${path}`)
  }))

  const data = { state: "ready", timezone: "Asia/Tokyo", rules: { revision: "rules-cached", items: [] }, plans: { revision: "plans-cached", items: [] }, projects: [], refetch: vi.fn() } as unknown as AllocationAPIData
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  function SaveButtons() {
    const actions = useContext(AllocationActionContext)!
    return <>
      <button onClick={() => void actions.savePlanParts({ revisions: { rules: "rules-v1", plans: "plans-v1" }, rules: { items: [] }, plans: { items: [] } })}>save both</button>
      <button onClick={() => void actions.savePlanParts({ revisions: { rules: "rules-v1", plans: "plans-v1" }, rules: { items: [] } })}>save rules</button>
      <button onClick={() => void actions.savePlanParts({ revisions: { rules: "rules-v1", plans: "plans-v1" }, plans: { items: [] } })}>save plans</button>
    </>
  }
  const tree = (apiData: AllocationAPIData) => <QueryClientProvider client={client}>
    <LanguageProviderContext.Provider value={{ language: "ja", setLanguage: vi.fn() }}>
      <AllocationAPIDataContext.Provider value={apiData}>
        <AllocationActionProvider><SaveButtons /></AllocationActionProvider>
      </AllocationAPIDataContext.Provider>
    </LanguageProviderContext.Provider>
  </QueryClientProvider>
  const view = render(tree(data))
  view.rerender(tree({ ...data, rules: { revision: "rules-v1", items: [] }, plans: { revision: "plans-v1", items: [] } } as unknown as AllocationAPIData))

  await act(async () => fireEvent.click(screen.getByRole("button", { name: "save both" })))
  expect(calls.slice(0, 2)).toEqual([
    { path: "/api/allocation-rules", ifMatch: '"rules-v1"' },
    { path: "/api/allocation-plans", ifMatch: '"plans-v1"' },
  ])

  await act(async () => fireEvent.click(screen.getByRole("button", { name: "save plans" })))
  await act(async () => fireEvent.click(screen.getByRole("button", { name: "save rules" })))
  expect(calls.slice(2)).toEqual([
    { path: "/api/allocation-plans", ifMatch: '"plans-v1"' },
    { path: "/api/allocation-rules", ifMatch: '"rules-v2"' },
  ])
})

