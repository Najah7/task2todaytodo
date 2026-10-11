import { afterEach, expect, test, vi } from "vitest"
import { cleanup, render, screen } from "@testing-library/react"
import type { UseQueryResult } from "@tanstack/react-query"
import type { RestInboxListResponse } from "~/api/generated/inbox"
import type { RestProjectResponse } from "~/api/generated/projects"
import type { RestUserResponse } from "~/api/generated/auth"
import { LanguageProviderContext } from "~/features/i18n/languageContext"
import { InboxAPIDataContext } from "~/features/inbox/providers/InboxAPIDataProvider/context"
import type { InboxAPIData } from "~/features/inbox/providers/InboxAPIDataProvider/context"
import { InboxActionContext } from "~/features/inbox/providers/InboxActionProvider/context"
import type { InboxActions } from "~/features/inbox/providers/InboxActionProvider/context"
import { InboxNavigationContext } from "~/features/inbox/providers/InboxNavigationProvider/context"
import type { InboxNavigation } from "~/features/inbox/providers/InboxNavigationProvider/context"
import { ListInboxType } from "~/api/generated/inbox"
import { InboxPageContent } from "."

afterEach(cleanup)

const inbox = {
  counts: { all: 1, task: 0, schedule: 1 },
  items: [{
    created_at: "2026-10-12T00:00:00Z",
    id: "schedule-1",
    project_id: null,
    project_name: null,
    schedule: {
      all_day: false,
      assignee_id: "user-1",
      blueprint_id: "blueprint-1",
      can_assign: false,
      can_update: true,
      can_update_following: false,
      completed: false,
      description: "",
      end_at: "2026-10-12T10:00:00Z",
      location: "",
      occurrence_date: "2026-10-12",
      occurrence_id: null,
      revision: 1,
      skipped: false,
      source_revision_id: "revision-1",
      start_at: "2026-10-12T09:00:00Z",
      timezone: "Asia/Tokyo",
    },
    task: null,
    title: "Standup",
    type: "schedule" as const,
  }],
  next_page_token: "",
  previous_page_token: "",
} satisfies RestInboxListResponse

const query = <T,>(overrides: Record<string, unknown> = {}) => ({
  data: undefined,
  error: null,
  isError: false,
  isLoading: false,
  isPending: false,
  refetch: vi.fn(),
  ...overrides,
}) as unknown as UseQueryResult<T>

const actions: InboxActions = {
  busy: false,
  complete: vi.fn(),
  createSchedule: vi.fn(),
  createTask: vi.fn(),
  retry: vi.fn(),
  retryOptions: vi.fn(),
}

const navigation: InboxNavigation = {
  state: { type: ListInboxType.all, projectId: "all", title: "" },
  currentURL: "/inbox",
  selectType: vi.fn(),
  selectProject: vi.fn(),
  changeTitle: vi.fn(),
  first: vi.fn(),
  previous: vi.fn(),
  next: vi.fn(),
  editTask: vi.fn(),
  editSchedule: vi.fn(),
  editItem: vi.fn(),
}

function renderInbox(data: InboxAPIData) {
  return render(
    <LanguageProviderContext.Provider value={{ language: "en", setLanguage: vi.fn() }}>
      <InboxAPIDataContext.Provider value={data}>
        <InboxActionContext.Provider value={actions}>
          <InboxNavigationContext.Provider value={navigation}>
            <InboxPageContent />
          </InboxNavigationContext.Provider>
        </InboxActionContext.Provider>
      </InboxAPIDataContext.Provider>
    </LanguageProviderContext.Provider>,
  )
}

test("waits for the user timezone before rendering cached schedule rows", () => {
  const data: InboxAPIData = {
    state: "loading",
    inbox,
    inboxQuery: query<RestInboxListResponse>({ data: inbox }),
    projects: [],
    projectsQuery: query<RestProjectResponse[]>(),
    userQuery: query<RestUserResponse>({ isPending: true }),
    timezone: "",
    userId: "",
  }
  const view = renderInbox(data)

  expect(screen.getByRole("status").textContent).toContain("Loading inbox")
  expect(screen.queryByText("Standup")).toBeNull()

  view.rerender(
    <LanguageProviderContext.Provider value={{ language: "en", setLanguage: vi.fn() }}>
      <InboxAPIDataContext.Provider value={{
        ...data,
        state: "ready",
        user: { user_id: "user-1", timezone: "Asia/Tokyo" } as RestUserResponse,
        userQuery: query<RestUserResponse>({ data: { user_id: "user-1", timezone: "Asia/Tokyo" } }),
        timezone: "Asia/Tokyo",
        userId: "user-1",
      }}>
        <InboxActionContext.Provider value={actions}>
          <InboxNavigationContext.Provider value={navigation}>
            <InboxPageContent />
          </InboxNavigationContext.Provider>
        </InboxActionContext.Provider>
      </InboxAPIDataContext.Provider>
    </LanguageProviderContext.Provider>,
  )

  expect(screen.getByRole("list", { name: "Inbox list" }).textContent).toContain("Standup")
})
