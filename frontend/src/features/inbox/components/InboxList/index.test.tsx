import { afterEach, expect, test, vi } from "vitest"
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react"
import { QueryClient, QueryClientProvider, type UseQueryResult } from "@tanstack/react-query"
import type { RestActionItemResponse } from "~/api/generated/tasks"
import type { RestInboxItemResponse, RestInboxListResponse } from "~/api/generated/inbox"
import { getGetTodayTodoQueryKey, type RestTodoItemResponse } from "~/api/generated/todo"
import { LanguageProviderContext } from "~/features/i18n/languageContext"
import { InboxAPIDataContext, type InboxAPIData } from "~/features/inbox/providers/InboxAPIDataProvider/context"
import { InboxActionContext, type InboxActions } from "~/features/inbox/providers/InboxActionProvider/context"
import { InboxNavigationContext, type InboxNavigation } from "~/features/inbox/providers/InboxNavigationProvider/context"
import InboxList from "."

const { getActionItems } = vi.hoisted(() => ({ getActionItems: vi.fn() }))
vi.mock("~/features/task/queries", () => ({ getAllTaskActionItems: getActionItems }))

afterEach(() => {
  cleanup()
  getActionItems.mockReset()
})

const taskItem: RestInboxItemResponse = {
  created_at: "2026-10-12T00:00:00Z",
  id: "task-1",
  project_id: null,
  project_name: null,
  schedule: null,
  task: {
    action_item_completed_count: 0,
    action_item_count: 1,
    actual_minutes: null,
    assignee_id: "user-1",
    can_add_action_item: true,
    can_add_to_today: false,
    can_assign: false,
    can_update: true,
    description: "",
    due_date: null,
    estimate_editable: false,
    estimate_source: "action_items",
    estimated_minutes: 30,
    id: "task-1",
    priority: "medium",
    progress: 0,
    remaining_days: null,
    revision: 1,
    status: "open",
    started: false,
  },
  title: "Prepare report",
  type: "task",
}

const directTaskItem: RestInboxItemResponse = {
  ...taskItem,
  title: "Read contract",
  task: { ...taskItem.task!, action_item_count: 0, can_add_to_today: true, priority: "high", estimate_editable: true },
}

const scheduleItem: RestInboxItemResponse = {
  created_at: "2026-10-12T00:00:00Z",
  id: "schedule-1",
  project_id: null,
  project_name: null,
  schedule: {
    all_day: false, assignee_id: "user-1", can_assign: true, can_update: false, can_update_following: true,
    completed: false, description: "", end_at: "2026-10-13T10:00:00Z", location: "", occurrence_date: "2026-10-13",
    occurrence_id: null, revision: 2, rule_id: "rule-1", skipped: false, source_revision_id: "rev-1",
    start_at: "2026-10-13T09:00:00Z", timezone: "Asia/Tokyo",
  },
  task: null,
  title: "Team review",
  type: "schedule",
}

function renderList(items: RestInboxItemResponse[], complete = vi.fn().mockResolvedValue(undefined), todayItems: RestTodoItemResponse[] = []) {
  const inbox: RestInboxListResponse = { counts: { all: items.length, task: 1, schedule: 1 }, items, next_page_token: "", previous_page_token: "" }
  const query = { data: inbox, isPending: false, isLoading: false, error: null } as unknown as UseQueryResult<RestInboxListResponse>
  const projectsQuery = { data: [], isPending: false, error: null } as unknown as UseQueryResult<never[]>
  const userQuery = { data: { user_id: "user-1", timezone: "UTC" }, isPending: false, error: null } as unknown as UseQueryResult<never>
  const data: InboxAPIData = { state: "ready", inbox, inboxQuery: query, projects: [], projectsQuery, user: { user_id: "user-1", timezone: "UTC" }, userQuery, timezone: "UTC", userId: "user-1" }
const completeActionItem = vi.fn().mockResolvedValue(undefined)
const reopenActionItem = vi.fn().mockResolvedValue(undefined)
const addActionItemToToday = vi.fn().mockResolvedValue(undefined)
const addTaskToToday = vi.fn().mockResolvedValue(undefined)
const actions: InboxActions = { busy: false, createTask: vi.fn(), createSchedule: vi.fn(), complete, completeActionItem, reopenActionItem, addActionItemToToday, addTaskToToday, retry: vi.fn(), retryOptions: vi.fn() }
  const navigation: InboxNavigation = { state: { type: "all", projectId: "all", title: "" }, currentURL: "/inbox", selectType: vi.fn(), selectProject: vi.fn(), changeTitle: vi.fn(), first: vi.fn(), previous: vi.fn(), next: vi.fn(), editTask: vi.fn(), editSchedule: vi.fn(), editItem: vi.fn() }
  const client = new QueryClient({ defaultOptions: { queries: { retry: false, staleTime: Infinity } } })
  client.setQueryData(getGetTodayTodoQueryKey(), { items: todayItems, timezone: "UTC" })
  render(<QueryClientProvider client={client}><LanguageProviderContext.Provider value={{ language: "en", setLanguage: vi.fn() }}>
    <InboxAPIDataContext.Provider value={data}><InboxActionContext.Provider value={actions}><InboxNavigationContext.Provider value={navigation}><InboxList /></InboxNavigationContext.Provider></InboxActionContext.Provider></InboxAPIDataContext.Provider>
  </LanguageProviderContext.Provider></QueryClientProvider>)
  return { complete, completeActionItem, reopenActionItem, addActionItemToToday, addTaskToToday, navigation }
}

test("expands Task row to show child ActionItems and records actual time on completion", async () => {
  getActionItems.mockResolvedValue([{ id: "action-1", task_id: "task-1", title: "Draft introduction", estimated_minutes: 30, completed: false } satisfies RestActionItemResponse])
  const { completeActionItem } = renderList([taskItem])

  expect(getActionItems).not.toHaveBeenCalled()
  fireEvent.click(screen.getByText("Show ActionItems"))
  expect(await screen.findByText("Draft introduction")).toBeTruthy()
  expect(getActionItems).toHaveBeenCalledWith("task-1")
  fireEvent.click(screen.getByRole("button", { name: "Complete Draft introduction" }))
  fireEvent.change(screen.getByLabelText("Actual time (minutes)"), { target: { value: "0" } })
  fireEvent.click(screen.getByRole("button", { name: "Save time and complete" }))
  await waitFor(() => expect(completeActionItem).toHaveBeenCalledWith("task-1", expect.objectContaining({ id: "action-1" }), 0))
})

test("shows the first current Today time and ignores null-time history on Inbox child rows", async () => {
  getActionItems.mockResolvedValue([{ id: "action-1", task_id: "task-1", title: "Draft introduction", estimated_minutes: 30, completed: false } satisfies RestActionItemResponse])
  renderList([taskItem], vi.fn().mockResolvedValue(undefined), [
    { kind: "action_item", action_item_id: "action-1", scheduled_start: "2026-10-12T09:00:00Z" },
    { kind: "action_item", action_item_id: "action-1", history: true, scheduled_start: null },
    { kind: "action_item", action_item_id: "action-1", scheduled_start: "2026-10-12T10:00:00Z" },
  ])
  fireEvent.click(screen.getByText("Show ActionItems"))
  await screen.findByText("Draft introduction")
  expect(screen.getByText("09:00")).toBeTruthy()
  expect(screen.queryByText("10:00")).toBeNull()
})

test("completes editable Task rows and exposes child expansion without a Task-level Today action", () => {
  const complete = vi.fn().mockResolvedValue(undefined)
  renderList([taskItem], complete)
  fireEvent.click(screen.getByRole("button", { name: "Complete Prepare report" }))
  expect(complete).toHaveBeenCalledWith(taskItem)
  expect(screen.getByText("Show ActionItems")).toBeTruthy()
  expect(screen.queryByRole("button", { name: "Add to Today" })).toBeNull()
})

test("childless Inbox Task adds directly to Today and completion records optional actual time", async () => {
  const complete = vi.fn().mockResolvedValue(undefined)
  const { addTaskToToday } = renderList([directTaskItem], complete)
  fireEvent.click(screen.getByRole("button", { name: "Add to today" }))
  expect(addTaskToToday).toHaveBeenCalledWith("task-1", "high")
  fireEvent.click(screen.getByRole("button", { name: "Complete Read contract" }))
  fireEvent.change(screen.getByLabelText("Actual time (minutes)"), { target: { value: "0" } })
  fireEvent.click(screen.getByRole("button", { name: "Save time and complete" }))
  await waitFor(() => expect(complete).toHaveBeenCalledWith(directTaskItem, 0))
})

test("reopens a completed child ActionItem directly", async () => {
  getActionItems.mockResolvedValue([{ id: "action-1", task_id: "task-1", title: "Draft introduction", estimated_minutes: 30, completed: true } satisfies RestActionItemResponse])
  const { reopenActionItem } = renderList([taskItem])
  fireEvent.click(screen.getByText("Show ActionItems"))
  await screen.findByText("Draft introduction")
  fireEvent.click(screen.getByRole("button", { name: "Reopen Draft introduction" }))
  expect(reopenActionItem).toHaveBeenCalledWith("task-1", expect.objectContaining({ id: "action-1" }))
})

test("adds only the child ActionItem to today's list", async () => {
  getActionItems.mockResolvedValue([{ id: "action-1", task_id: "task-1", title: "Draft introduction", estimated_minutes: 30, completed: false } satisfies RestActionItemResponse])
  const { addActionItemToToday } = renderList([taskItem])
  fireEvent.click(screen.getByText("Show ActionItems"))
  await screen.findByText("Draft introduction")
  fireEvent.click(screen.getByRole("button", { name: "Add to today" }))
  await waitFor(() => expect(addActionItemToToday).toHaveBeenCalledWith("task-1", expect.objectContaining({ id: "action-1" })))
  expect(screen.getByRole("button", { name: "Add to today" }).closest("li")).toBeTruthy()
})

test("keeps actual-minute draft open when ActionItem completion fails", async () => {
  getActionItems.mockResolvedValue([{ id: "action-1", task_id: "task-1", title: "Draft introduction", estimated_minutes: 30, completed: false } satisfies RestActionItemResponse])
  const { completeActionItem } = renderList([taskItem])
  completeActionItem.mockRejectedValueOnce(new Error("network failure"))
  fireEvent.click(screen.getByText("Show ActionItems"))
  await screen.findByText("Draft introduction")
  fireEvent.click(screen.getByRole("button", { name: "Complete Draft introduction" }))
  const input = screen.getByLabelText("Actual time (minutes)") as HTMLInputElement
  fireEvent.change(input, { target: { value: "25" } })
  fireEvent.click(screen.getByRole("button", { name: "Save time and complete" }))
  await waitFor(() => expect(completeActionItem).toHaveBeenCalled())
  expect((screen.getByLabelText("Actual time (minutes)") as HTMLInputElement).value).toBe("25")
  expect(screen.getByRole("dialog")).toBeTruthy()
})

test("allows editing a following-only Schedule while disabling completion", () => {
  const complete = vi.fn()
  const { navigation } = renderList([scheduleItem], complete)
  expect(screen.getByRole("button", { name: "Edit Team review" }).hasAttribute("disabled")).toBe(false)
  expect(screen.getByRole("button", { name: "Complete Team review" }).hasAttribute("disabled")).toBe(true)
  fireEvent.click(screen.getByRole("button", { name: "Edit Team review" }))
  expect(navigation.editItem).toHaveBeenCalledWith(scheduleItem)
  expect(complete).not.toHaveBeenCalled()
})

test("keeps read-only Task details visible but disables child mutations", async () => {
  getActionItems.mockResolvedValue([{ id: "action-1", task_id: "task-1", title: "Draft introduction", estimated_minutes: 30, completed: false } satisfies RestActionItemResponse])
  const readOnly = { ...taskItem, task: { ...taskItem.task!, can_update: false } }
  renderList([readOnly])
  expect(screen.getByRole("button", { name: "Complete Prepare report" }).hasAttribute("disabled")).toBe(true)
  expect(screen.queryByRole("button", { name: "Edit Prepare report" })).toBeNull()
  fireEvent.click(screen.getByRole("button", { name: "Show ActionItems for Prepare report" }))
  await screen.findByText("Draft introduction")
  expect(screen.getByRole("button", { name: "Complete Draft introduction" }).hasAttribute("disabled")).toBe(true)
  expect(screen.getByRole("button", { name: "Add to today" }).hasAttribute("disabled")).toBe(true)
})
