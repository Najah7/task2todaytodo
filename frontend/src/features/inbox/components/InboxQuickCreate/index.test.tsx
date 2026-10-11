import type { UseQueryResult } from "@tanstack/react-query"
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react"
import { afterEach, describe, expect, it, vi } from "vitest"
import type { RestInboxListResponse } from "~/api/generated/inbox"
import { InboxActionContext, type InboxActions } from "~/features/inbox/providers/InboxActionProvider/context"
import { InboxAPIDataContext, type InboxAPIData } from "~/features/inbox/providers/InboxAPIDataProvider/context"
import { InboxNavigationContext, type InboxNavigation } from "~/features/inbox/providers/InboxNavigationProvider/context"
import { LanguageProviderContext } from "~/features/i18n/languageContext"
import { emptyInboxQuickCreateValues } from "./schema"
import InboxQuickCreate from "./index"

const inbox: RestInboxListResponse = { counts: { all: 0, task: 0, schedule: 0 }, items: [], next_page_token: "", previous_page_token: "" }

afterEach(cleanup)

function renderQuickCreate({ busy = false, createTask = vi.fn().mockResolvedValue(undefined), createSchedule = vi.fn().mockResolvedValue(undefined) }: {
  busy?: boolean
  createTask?: InboxActions["createTask"]
  createSchedule?: InboxActions["createSchedule"]
} = {}) {
  const projectsQuery = { data: [], isPending: false, error: null } as unknown as UseQueryResult<never[]>
  const userQuery = { data: { user_id: "user-1", timezone: "Asia/Tokyo" }, isPending: false, error: null } as unknown as UseQueryResult<never>
  const inboxQuery = { data: inbox, isPending: false, isLoading: false, error: null } as unknown as UseQueryResult<RestInboxListResponse>
  const data: InboxAPIData = { state: "ready", inbox, inboxQuery, projects: [], projectsQuery, user: { user_id: "user-1", timezone: "Asia/Tokyo" }, userQuery, timezone: "Asia/Tokyo", userId: "user-1" }
  const actions: InboxActions = { busy, createTask, createSchedule, complete: vi.fn(), retry: vi.fn(), retryOptions: vi.fn() }
  const navigation: InboxNavigation = {
    state: { type: "all", projectId: "all", title: "" }, currentURL: "/inbox", selectType: vi.fn(), selectProject: vi.fn(), changeTitle: vi.fn(), first: vi.fn(), previous: vi.fn(), next: vi.fn(), editTask: vi.fn(), editSchedule: vi.fn(), editItem: vi.fn(),
  }
  render(<LanguageProviderContext.Provider value={{ language: "ja", setLanguage: vi.fn() }}>
    <InboxAPIDataContext.Provider value={data}>
      <InboxActionContext.Provider value={actions}>
        <InboxNavigationContext.Provider value={navigation}><InboxQuickCreate /></InboxNavigationContext.Provider>
      </InboxActionContext.Provider>
    </InboxAPIDataContext.Provider>
  </LanguageProviderContext.Provider>)
  return { createTask, createSchedule, navigation }
}

describe("InboxQuickCreate", () => {
  it("validates required title and submits task with optional project", async () => {
    const { createTask } = renderQuickCreate()
    fireEvent.click(screen.getByRole("button", { name: "追加" }))
    expect(await screen.findByText("タスク名を入力してください。")).toBeTruthy()
    expect(createTask).not.toHaveBeenCalled()

    fireEvent.change(screen.getByRole("textbox", { name: "タスク名" }), { target: { value: "Read inbox" } })
    fireEvent.click(screen.getByRole("button", { name: "追加" }))
    await waitFor(() => expect(createTask).toHaveBeenCalledWith({ title: "Read inbox", projectId: "" }))
  })

  it("validates schedule date relationships before submit", async () => {
    const { createSchedule } = renderQuickCreate()
    fireEvent.click(screen.getByRole("tab", { name: "予定" }))
    fireEvent.change(screen.getByLabelText("開始日"), { target: { value: "2026-10-15" } })
    fireEvent.change(screen.getByLabelText("終了日"), { target: { value: "2026-10-14" } })
    fireEvent.change(screen.getByRole("textbox", { name: "予定名" }), { target: { value: "Dentist" } })
    fireEvent.click(screen.getByRole("button", { name: "追加" }))
    expect(await screen.findByText("終了日時は開始日時以降にしてください。")).toBeTruthy()
    expect(createSchedule).not.toHaveBeenCalled()
  })

  it("disables submit while busy", () => {
    renderQuickCreate({ busy: true })
    expect(screen.getByRole("button", { name: "追加中…" }).hasAttribute("disabled")).toBe(true)
  })

  it("keeps draft after a rejected create and resets only after success", async () => {
    const createTask = vi.fn().mockRejectedValueOnce(new Error("offline")).mockResolvedValueOnce(undefined)
    const { navigation } = renderQuickCreate({ createTask })
    const title = screen.getByRole("textbox", { name: "タスク名" }) as HTMLInputElement
    fireEvent.change(title, { target: { value: "Draft item" } })
    fireEvent.click(screen.getByRole("button", { name: "追加" }))
    await waitFor(() => expect(createTask).toHaveBeenCalledTimes(1))
    expect(title.value).toBe("Draft item")

    fireEvent.click(screen.getByRole("button", { name: "追加" }))
    await waitFor(() => expect(createTask).toHaveBeenCalledTimes(2))
    await waitFor(() => expect(title.value).toBe(emptyInboxQuickCreateValues.title))
    expect(navigation.first).toHaveBeenCalledTimes(1)
  })
})
