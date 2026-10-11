import type { UseQueryResult } from "@tanstack/react-query"
import { cleanup, fireEvent, render, screen } from "@testing-library/react"
import { afterEach, describe, expect, it, vi } from "vitest"
import type { RestInboxListResponse } from "~/api/generated/inbox"
import { InboxAPIDataContext, type InboxAPIData } from "~/features/inbox/providers/InboxAPIDataProvider/context"
import { InboxNavigationContext, type InboxNavigation } from "~/features/inbox/providers/InboxNavigationProvider/context"
import { LanguageProviderContext } from "~/features/i18n/languageContext"
import InboxPagination from "./index"

describe("InboxPagination", () => {
  afterEach(cleanup)
  it("recovers an empty cursor page by returning to first page", () => {
    const inbox: RestInboxListResponse = { counts: { all: 12, task: 9, schedule: 3 }, items: [], next_page_token: "", previous_page_token: "previous" }
    const inboxQuery = { data: inbox, isPending: false, isLoading: false, error: null } as unknown as UseQueryResult<RestInboxListResponse>
    const projectsQuery = { data: [], isPending: false, error: null } as unknown as UseQueryResult<never[]>
    const userQuery = { data: { user_id: "user-1", timezone: "Asia/Tokyo" }, isPending: false } as unknown as UseQueryResult<never>
    const data: InboxAPIData = { state: "ready", inbox, inboxQuery, projects: [], projectsQuery, user: { user_id: "user-1", timezone: "Asia/Tokyo" }, userQuery, timezone: "Asia/Tokyo", userId: "user-1" }
    const first = vi.fn()
    const navigation: InboxNavigation = { state: { type: "all", projectId: "all", title: "", pageToken: "stale-token" }, currentURL: "/inbox?page_token=stale-token", selectType: vi.fn(), selectProject: vi.fn(), changeTitle: vi.fn(), first, previous: vi.fn(), next: vi.fn(), editTask: vi.fn(), editSchedule: vi.fn(), editItem: vi.fn() }
    render(<LanguageProviderContext.Provider value={{ language: "en", setLanguage: vi.fn() }}><InboxAPIDataContext.Provider value={data}><InboxNavigationContext.Provider value={navigation}><InboxPagination /></InboxNavigationContext.Provider></InboxAPIDataContext.Provider></LanguageProviderContext.Provider>)
    expect(screen.getByText("This page has no items. Return to the first page.")).toBeTruthy()
    fireEvent.click(screen.getByRole("button", { name: "First page" }))
    expect(first).toHaveBeenCalledOnce()
    expect(screen.getByRole("button", { name: "Previous" }).hasAttribute("disabled")).toBe(false)
    expect(screen.getByRole("button", { name: "Next" }).hasAttribute("disabled")).toBe(true)
  })
})
