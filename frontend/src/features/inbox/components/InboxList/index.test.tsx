import type { UseQueryResult } from "@tanstack/react-query"
import { cleanup, fireEvent, render, screen } from "@testing-library/react"
import { afterEach, describe, expect, it, vi } from "vitest"
import type { RestInboxItemResponse, RestInboxListResponse } from "~/api/generated/inbox"
import { InboxActionContext, type InboxActions } from "~/features/inbox/providers/InboxActionProvider/context"
import { InboxAPIDataContext, type InboxAPIData } from "~/features/inbox/providers/InboxAPIDataProvider/context"
import { InboxNavigationContext, type InboxNavigation } from "~/features/inbox/providers/InboxNavigationProvider/context"
import { LanguageProviderContext } from "~/features/i18n/languageContext"
import InboxList from "./index"

const taskItem: RestInboxItemResponse = {
  id: "task-1", type: "task", title: "Prepare report", project_id: null, project_name: null, created_at: "2026-10-12T00:00:00Z",
  task: { id: "task-1", assignee_id: "user-1", can_assign: true, can_update: true, description: "", due_date: null, estimated_minutes: null, actual_minutes: null, estimate_source: "manual", priority: "low", progress: 0, remaining_days: null, action_item_completed_count: 0, action_item_count: 1, revision: 2, status: "open" },
  schedule: null,
}

const scheduleItem: RestInboxItemResponse = {
  id: "schedule-1", type: "schedule", title: "Team review", project_id: null, project_name: null, created_at: "2026-10-12T00:00:00Z",
  task: null,
  schedule: { all_day: false, assignee_id: "user-1", blueprint_id: "blueprint-1", can_assign: true, can_update: false, can_update_following: true, completed: false, description: "", end_at: "2026-10-13T10:00:00Z", location: "", occurrence_date: "2026-10-13", occurrence_id: null, revision: 2, skipped: false, source_revision_id: "rev-1", start_at: "2026-10-13T09:00:00Z", timezone: "Asia/Tokyo" },
}

afterEach(cleanup)

function renderList(items: RestInboxItemResponse[], complete = vi.fn().mockResolvedValue(undefined)) {
  const inbox: RestInboxListResponse = { counts: { all: items.length, task: 1, schedule: 1 }, items, next_page_token: "", previous_page_token: "" }
  const query = { data: inbox, isPending: false, isLoading: false, error: null } as unknown as UseQueryResult<RestInboxListResponse>
  const projectsQuery = { data: [], isPending: false, error: null } as unknown as UseQueryResult<never[]>
  const userQuery = { data: { user_id: "user-1", timezone: "America/Los_Angeles" }, isPending: false, error: null } as unknown as UseQueryResult<never>
  const data: InboxAPIData = { state: "ready", inbox, inboxQuery: query, projects: [], projectsQuery, user: { user_id: "user-1", timezone: "America/Los_Angeles" }, userQuery, timezone: "America/Los_Angeles", userId: "user-1" }
  const actions: InboxActions = { busy: false, createTask: vi.fn(), createSchedule: vi.fn(), complete, retry: vi.fn(), retryOptions: vi.fn() }
  const navigation: InboxNavigation = { state: { type: "all", projectId: "all", title: "" }, currentURL: "/inbox", selectType: vi.fn(), selectProject: vi.fn(), changeTitle: vi.fn(), first: vi.fn(), previous: vi.fn(), next: vi.fn(), editTask: vi.fn(), editSchedule: vi.fn(), editItem: vi.fn() }
  render(<LanguageProviderContext.Provider value={{ language: "en", setLanguage: vi.fn() }}>
    <InboxAPIDataContext.Provider value={data}><InboxActionContext.Provider value={actions}><InboxNavigationContext.Provider value={navigation}><InboxList /></InboxNavigationContext.Provider></InboxActionContext.Provider></InboxAPIDataContext.Provider>
  </LanguageProviderContext.Provider>)
  return { complete, navigation }
}

describe("InboxList", () => {
  it("completes editable tasks and keeps Today action disabled", () => {
    const { complete } = renderList([taskItem])
    fireEvent.click(screen.getByRole("button", { name: "Complete Prepare report" }))
    expect(complete).toHaveBeenCalledWith(taskItem)
    expect(screen.getByRole("button", { name: "Add to Today" }).hasAttribute("disabled")).toBe(true)
  })

  it("allows editing a following-only schedule while disabling completion", () => {
    const { complete, navigation } = renderList([scheduleItem])
    expect(screen.getByRole("button", { name: "Edit Team review" }).hasAttribute("disabled")).toBe(false)
    expect(screen.getByRole("button", { name: "Complete Team review" }).hasAttribute("disabled")).toBe(true)
    fireEvent.click(screen.getByRole("button", { name: "Edit Team review" }))
    expect(navigation.editItem).toHaveBeenCalledWith(scheduleItem)
    expect(complete).not.toHaveBeenCalled()
  })

  it("disables completion and hides edit for read-only tasks", () => {
    const readOnly = { ...taskItem, task: { ...taskItem.task!, can_update: false } }
    renderList([readOnly])
    expect(screen.getByRole("button", { name: "Complete Prepare report" }).hasAttribute("disabled")).toBe(true)
    expect(screen.queryByRole("button", { name: "Edit Prepare report" })).toBeNull()
  })
})
