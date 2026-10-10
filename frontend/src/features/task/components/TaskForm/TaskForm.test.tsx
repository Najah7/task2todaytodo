import { act, cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react"
import { afterEach, expect, test, vi } from "vitest"
import { useState } from "react"
import { createMemoryRouter, RouterProvider } from "react-router"
import { LanguageProviderContext } from "~/features/i18n/languageContext"
import { TaskActionContext, type TaskActions, type TaskFormReloadSnapshot, type TaskFormSaveResult } from "~/features/task/providers/TaskActionProvider/context"
import { TaskAPIDataContext, type TaskAPIData } from "~/features/task/providers/TaskAPIDataProvider/context"
import { TaskNavigationContext, type TaskNavigation } from "~/features/task/providers/TaskNavigationProvider/context"
import { taskFormValues2actionItemSaveOperations, type TaskFormDirtyFields } from "~/features/task/converters/taskFormValues2updateRequest"
import { useTaskCreateForm } from "~/features/task/hooks/useTaskCreateForm"
import { useTaskEditForm } from "~/features/task/hooks/useTaskEditForm"
import TaskForm from "."
import { emptyTaskFormValues, type TaskFormValues } from "./schema"

afterEach(cleanup)

const emptyQuery = { data: undefined, error: null, isError: false, isLoading: false, refetch: vi.fn() }
const initialData = (): TaskAPIData => ({
  state: "ready",
  list: false,
  tasksQuery: emptyQuery,
  taskQuery: emptyQuery,
  actionItemsQuery: emptyQuery,
  projectsQuery: { ...emptyQuery, data: [] },
  optionsQuery: { ...emptyQuery, data: { priorities: [] } },
} as unknown as TaskAPIData)

function baseActions(submit: TaskActions["submit"], reloadLatest?: TaskActions["reloadLatest"]): TaskActions {
  return {
    busy: false,
    submit,
    saveComplete: vi.fn(),
    retry: vi.fn(async () => {}),
    retryOptions: vi.fn(async () => {}),
    updateActionItemCompletion: vi.fn(async () => {}),
    ...(reloadLatest ? { reloadLatest } : {}),
  }
}

const navigation: TaskNavigation = {
  state: { status: "open", dueFilter: "all", title: "", sortBy: "due_date", sortOrder: "asc" },
  create: vi.fn(), edit: vi.fn(), cancel: vi.fn(), afterSave: vi.fn(), selectStatus: vi.fn(),
  changeFilter: vi.fn(), changeSort: vi.fn(), first: vi.fn(), previous: vi.fn(), next: vi.fn(),
}

function renderHookForm({
  mode = "create",
  data = initialData(),
  submit = vi.fn(async () => ({ complete: true })),
  reloadLatest,
}: {
  mode?: "create" | "edit"
  data?: TaskAPIData
  submit?: TaskActions["submit"]
  reloadLatest?: TaskActions["reloadLatest"]
} = {}) {
  const actions = baseActions(submit, reloadLatest)
  let updateData: (next: TaskAPIData) => void = () => {}
  function FormRoute() {
    const [currentData, setCurrentData] = useState(data)
    updateData = setCurrentData
    return (
      <LanguageProviderContext.Provider value={{ language: "ja", setLanguage: vi.fn() }}>
        <TaskAPIDataContext.Provider value={currentData}>
          <TaskActionContext.Provider value={actions}>
            <TaskNavigationContext.Provider value={navigation}>{mode === "create" ? <CreateReady /> : <EditReady />}</TaskNavigationContext.Provider>
          </TaskActionContext.Provider>
        </TaskAPIDataContext.Provider>
      </LanguageProviderContext.Provider>
    )
  }
  const router = createMemoryRouter([
    { path: "/tasks/new", element: <FormRoute /> },
    { path: "/tasks/:id/edit", element: <FormRoute /> },
    { path: "/tasks", element: <p>Task list</p> },
  ], { initialEntries: [mode === "create" ? "/tasks/new" : "/tasks/task-1/edit"] })
  render(<RouterProvider router={router} />)
  return { router, actions, updateData: (next: TaskAPIData) => act(() => updateData(next)) }
}

function CreateReady() {
  const controller = useTaskCreateForm()
  return <TaskForm form={controller.form} {...controller.viewProps} />
}

function EditReady() {
  const controller = useTaskEditForm()
  return <TaskForm form={controller.form} {...controller.viewProps} />
}

function editData(values: Partial<TaskFormValues> = {}): TaskAPIData {
  const task = {
    id: "task-1", title: values.title ?? "Existing task", project_id: null, project_name: null,
    due_date: null, description: "", priority: "medium", manual_estimated_minutes: null,
    estimate_source: "manual", revision: 3, can_update: true,
  }
  const actionItems = (values.actionItems ?? []).map((item, index) => ({
    id: item.seriesId ?? `item-${index}`, series_id: item.seriesId ?? null,
    occurrence_date: item.occurrenceDate ?? "2026-10-10", title: item.title,
    estimated_minutes: 30, priority: item.priority, completed: false,
  }))
  return {
    ...initialData(), taskId: "task-1", task,
    taskQuery: { ...emptyQuery, data: { task } },
    actionItemsQuery: { ...emptyQuery, data: actionItems },
  } as unknown as TaskAPIData
}

test("keeps create draft and marks Task field returned by API", async () => {
  const submit = vi.fn().mockRejectedValue({ fieldErrors: { title: "tasks.form.invalidField" } })
  renderHookForm({ submit })
  const form = screen.getByRole("form")
  fireEvent.change(within(form).getByLabelText(/^タスク名/), { target: { value: "Keep this draft" } })
  fireEvent.click(within(form).getByRole("button", { name: "アクションアイテムを追加" }))
  fireEvent.change(within(form).getByLabelText(/^アクションアイテム名/), { target: { value: "Draft the report" } })
  fireEvent.change(within(form).getByLabelText("見積り 1"), { target: { value: "0:30" } })
  fireEvent.click(within(form).getByRole("button", { name: "追加する" }))
  expect(await within(form).findByText("入力内容を確認してください。")).toBeTruthy()
  expect((within(form).getByLabelText(/^タスク名/) as HTMLInputElement).value).toBe("Keep this draft")
  expect((within(form).getByLabelText(/^アクションアイテム名/) as HTMLInputElement).value).toBe("Draft the report")
  expect(submit).toHaveBeenCalledOnce()
})

test("retries only failed ActionItems after applying successful row IDs and defaults", async () => {
  let saveAttempt = 0
  let retryOperations: ReturnType<typeof taskFormValues2actionItemSaveOperations> = []
  const submit = vi.fn(async (values: TaskFormValues, dirtyFields: TaskFormDirtyFields): Promise<TaskFormSaveResult> => {
    const operations = taskFormValues2actionItemSaveOperations(values, dirtyFields)
    if (saveAttempt++ === 0) return {
      complete: false,
      taskSaved: true,
      actionItems: [
        { index: 0, key: values.actionItems[0]!.clientKey!, identity: { seriesId: "saved-item-1", occurrenceDate: "2026-10-10" }, savedFields: { title: "Saved item", estimatedMinutes: "0:30", priority: "medium" } },
        { index: 1, key: values.actionItems[1]!.clientKey!, error: "tasks.form.actionItemSaveError" },
      ],
    }
    retryOperations = operations
    return { complete: true }
  })
  renderHookForm({ submit })
  const form = screen.getByRole("form")
  fireEvent.change(within(form).getByLabelText(/^タスク名/), { target: { value: "Task with items" } })
  fireEvent.click(within(form).getByRole("button", { name: "アクションアイテムを追加" }))
  fireEvent.change(within(form).getByLabelText("アクションアイテム名 1"), { target: { value: "Saved item" } })
  fireEvent.change(within(form).getByLabelText("見積り 1"), { target: { value: "0:30" } })
  fireEvent.click(within(form).getByRole("button", { name: "アクションアイテムを追加" }))
  fireEvent.change(within(form).getByLabelText("アクションアイテム名 2"), { target: { value: "Retry item" } })
  fireEvent.click(within(form).getByRole("button", { name: "追加する" }))
  expect(await within(form).findByText("タスクは保存済みです。未保存のアクションアイテムだけ再試行してください。")).toBeTruthy()
  expect(await within(form).findByText("アクションアイテムを保存できませんでした。")).toBeTruthy()
  fireEvent.click(within(form).getByRole("button", { name: "追加する" }))
  await waitFor(() => expect(submit).toHaveBeenCalledTimes(2))
  expect(retryOperations).toHaveLength(1)
  expect(retryOperations[0]).toMatchObject({ type: "create", item: { title: "Retry item" } })
  expect(submit.mock.calls[1]![0].actionItems[0]).toMatchObject({ seriesId: "saved-item-1", title: "Saved item" })
})

test("does not validate an ActionItem row marked for deletion", async () => {
  const submit = vi.fn(async () => ({ complete: true }))
  renderHookForm({ submit })
  const form = screen.getByRole("form")
  fireEvent.change(within(form).getByLabelText(/^タスク名/), { target: { value: "Task" } })
  fireEvent.click(within(form).getByRole("button", { name: "アクションアイテムを追加" }))
  fireEvent.change(within(form).getByLabelText("見積り 1"), { target: { value: "not a duration" } })
  fireEvent.click(within(form).getByRole("button", { name: "アクションアイテム 1 を削除" }))
  fireEvent.click(within(form).getByRole("button", { name: "追加する" }))
  await waitFor(() => expect(submit).toHaveBeenCalledOnce())
  expect(within(form).queryByLabelText("見積り 1")).toBeNull()
})

test("shows unmapped API validation errors in the form", async () => {
  const submit = vi.fn().mockRejectedValue({ fieldErrors: { unexpected_field: "tasks.form.invalidField" } })
  renderHookForm({ submit })
  const form = screen.getByRole("form")
  fireEvent.change(within(form).getByLabelText(/^タスク名/), { target: { value: "Task" } })
  fireEvent.click(within(form).getByRole("button", { name: "追加する" }))
  expect(await within(form).findByText("タスクを保存できませんでした。入力内容を確認してください。"))
})

test("does not present a Task request failure as an ActionItem partial save", async () => {
  const submit = vi.fn().mockRejectedValue(new TypeError("network failure"))
  renderHookForm({ submit })
  const form = screen.getByRole("form")
  fireEvent.change(within(form).getByLabelText(/^タスク名/), { target: { value: "Keep draft" } })
  fireEvent.click(within(form).getByRole("button", { name: "追加する" }))
  await waitFor(() => expect(submit).toHaveBeenCalledOnce())
  expect(within(form).queryByText("一部の変更を保存できませんでした。エラー項目を修正または再試行してください。")).toBeNull()
  expect((within(form).getByLabelText(/^タスク名/) as HTMLInputElement).value).toBe("Keep draft")
})

test("asks before leaving a dirty Task draft and keeps it when user stays", async () => {
  const { router } = renderHookForm()
  const form = screen.getByRole("form")
  fireEvent.change(within(form).getByLabelText(/^タスク名/), { target: { value: "Unsaved task" } })
  fireEvent.click(screen.getByRole("link", { name: "タスク" }))
  const dialog = await screen.findByRole("alertdialog", { name: "変更を破棄しますか？" })
  fireEvent.click(within(dialog).getByRole("button", { name: "編集を続ける" }))
  expect(router.state.location.pathname).toBe("/tasks/new")
  expect((within(form).getByLabelText(/^タスク名/) as HTMLInputElement).value).toBe("Unsaved task")
})

test("background Task refetch does not replace edit draft", () => {
  const { updateData } = renderHookForm({ mode: "edit", data: editData() })
  const form = screen.getByRole("form")
  fireEvent.change(within(form).getByLabelText(/^タスク名/), { target: { value: "Keep this draft" } })
  updateData(editData({ title: "Background server update" }))
  expect((within(form).getByLabelText(/^タスク名/) as HTMLInputElement).value).toBe("Keep this draft")
})

test("reloads latest edit values and clears save feedback", async () => {
  const reloadLatest = vi.fn(async (): Promise<TaskFormReloadSnapshot> => ({ values: { ...emptyTaskFormValues, title: "Latest server task" } }))
  const submit = vi.fn().mockRejectedValue(Object.assign(new Error("conflict"), { conflict: true }))
  renderHookForm({ mode: "edit", data: editData(), submit, reloadLatest })
  const form = screen.getByRole("form")
  fireEvent.change(within(form).getByLabelText(/^タスク名/), { target: { value: "Local draft" } })
  fireEvent.click(within(form).getByRole("button", { name: "変更を保存" }))
  await waitFor(() => expect(within(form).getByText("別の変更が先に保存されています。最新の内容を読み直してから編集してください。入力中の変更は保持されています。")))
  fireEvent.click(within(form).getByRole("button", { name: "最新の内容を読み込む" }))
  const dialog = await screen.findByRole("alertdialog", { name: "最新の内容を読み込みますか？" })
  await act(async () => {
    fireEvent.click(within(dialog).getByRole("button", { name: "最新の内容を読み込む" }))
    await new Promise((resolve) => setTimeout(resolve, 0))
  })
  await waitFor(() => expect(reloadLatest).toHaveBeenCalledOnce())
  await waitFor(() => expect((within(screen.getByRole("form")).getByLabelText(/^タスク名/) as HTMLInputElement).value).toBe("Latest server task"))
})

test("keeps edit draft and reload confirmation open when latest reload fails", async () => {
  const reloadLatest = vi.fn().mockRejectedValue(new Error("network failure"))
  const submit = vi.fn().mockRejectedValue(Object.assign(new Error("conflict"), { conflict: true }))
  renderHookForm({ mode: "edit", data: editData(), submit, reloadLatest })
  const form = screen.getByRole("form")
  fireEvent.change(within(form).getByLabelText(/^タスク名/), { target: { value: "Keep my draft" } })
  fireEvent.click(within(form).getByRole("button", { name: "変更を保存" }))
  await waitFor(() => expect(within(form).getByText("別の変更が先に保存されています。最新の内容を読み直してから編集してください。入力中の変更は保持されています。")))
  fireEvent.click(within(form).getByRole("button", { name: "最新の内容を読み込む" }))
  const dialog = await screen.findByRole("alertdialog", { name: "最新の内容を読み込みますか？" })
  fireEvent.click(within(dialog).getByRole("button", { name: "最新の内容を読み込む" }))
  await waitFor(() => expect(reloadLatest).toHaveBeenCalledOnce())
  expect(screen.getByRole("alertdialog", { name: "最新の内容を読み込みますか？" })).toBeTruthy()
  expect((within(form).getByLabelText(/^タスク名/) as HTMLInputElement).value).toBe("Keep my draft")
})
