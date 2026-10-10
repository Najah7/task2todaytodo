import { cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react"
import { afterEach, expect, test, vi } from "vitest"
import { createMemoryRouter, RouterProvider } from "react-router"
import { LanguageProviderContext } from "~/features/i18n/languageContext"
import TaskForm from "."
import { taskFormValues2actionItemSaveOperations, type TaskFormDirtyFields } from "~/features/task/converters/taskFormValues2updateRequest"
import type { TaskFormValues } from "./schema"

afterEach(cleanup)

test("keeps the create draft and marks the Task field returned by the API", async () => {
  const onClose = vi.fn()
  const onSubmit = vi.fn().mockRejectedValue({ fieldErrors: { title: "invalid" } })
  const router = createMemoryRouter([{
    path: "*",
    element: (
      <LanguageProviderContext.Provider value={{ language: "ja", setLanguage: vi.fn() }}>
        <TaskForm mode="create" projectOptions={[]} priorityOptions={[]} onCancel={onClose} onSubmit={onSubmit} />
      </LanguageProviderContext.Provider>
    ),
  }], { initialEntries: ["/tasks/new"] })
  render(<RouterProvider router={router} />)

  const form = screen.getByRole("form")
  fireEvent.change(within(form).getByLabelText(/^タスク名/), { target: { value: "Keep this draft" } })
  fireEvent.click(within(form).getByRole("button", { name: "アクションアイテムを追加" }))
  fireEvent.change(within(form).getByLabelText(/^アクションアイテム名/), { target: { value: "Draft the report" } })
  fireEvent.change(within(form).getByLabelText("見積り 1"), { target: { value: "0:30" } })
  fireEvent.click(within(form).getByRole("button", { name: "追加する" }))

  const taskTitle = within(form).getByLabelText(/^タスク名/)
  await waitFor(() => expect(taskTitle.getAttribute("aria-invalid")).toBe("true"))
  expect(await within(form).findByText("入力内容を確認してください。")).toBeTruthy()
  expect((within(form).getByLabelText(/^タスク名/) as HTMLInputElement).value).toBe("Keep this draft")
  expect((within(form).getByLabelText(/^アクションアイテム名/) as HTMLInputElement).value).toBe("Draft the report")
  expect((within(form).getByLabelText("見積り 1") as HTMLInputElement).value).toBe("0:30")
  expect(taskTitle.getAttribute("aria-invalid")).toBe("true")
  expect(within(form).getByLabelText(/^アクションアイテム名/).getAttribute("aria-invalid")).toBe("false")
  expect(onSubmit).toHaveBeenCalledOnce()
  expect(onClose).not.toHaveBeenCalled()
  expect(screen.getByRole("heading", { name: "タスクを追加" })).toBeTruthy()
})

test("retries only failed ActionItems after applying successful row IDs and defaults", async () => {
  let saveAttempt = 0
  let retryOperations: ReturnType<typeof taskFormValues2actionItemSaveOperations> = []
  const onSubmit = vi.fn(async (values: TaskFormValues, dirtyFields: TaskFormDirtyFields) => {
    const operations = taskFormValues2actionItemSaveOperations(values, dirtyFields)
    if (saveAttempt++ === 0) {
      return {
        complete: false,
        taskSaved: true,
        actionItems: [
          { index: 0, key: values.actionItems[0]!.clientKey!, identity: { seriesId: "saved-item-1", occurrenceDate: "2026-10-10" }, savedFields: { title: "Saved item", estimatedMinutes: "0:30", priority: "medium" } },
          { index: 1, key: values.actionItems[1]!.clientKey!, error: "tasks.form.actionItemSaveError" as const },
        ],
      }
    }
    retryOperations = operations
    return { complete: true }
  })
  const router = createMemoryRouter([{
    path: "*",
    element: (
      <LanguageProviderContext.Provider value={{ language: "ja", setLanguage: vi.fn() }}>
        <TaskForm mode="create" projectOptions={[]} priorityOptions={[{ value: "medium", label: "Medium" }]} onSubmit={onSubmit} />
      </LanguageProviderContext.Provider>
    ),
  }], { initialEntries: ["/tasks/new"] })
  render(<RouterProvider router={router} />)

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

  await waitFor(() => expect(onSubmit).toHaveBeenCalledTimes(2))
  expect(retryOperations).toHaveLength(1)
  expect(retryOperations[0]).toMatchObject({ type: "create", item: { title: "Retry item" } })
  expect(onSubmit.mock.calls[1]![0].actionItems[0]).toMatchObject({ seriesId: "saved-item-1", title: "Saved item" })
})

test("does not validate an action item row marked for deletion", async () => {
  const onSubmit = vi.fn(async () => ({ complete: true }))
  const router = createMemoryRouter([{
    path: "*",
    element: (
      <LanguageProviderContext.Provider value={{ language: "ja", setLanguage: vi.fn() }}>
        <TaskForm mode="create" projectOptions={[]} priorityOptions={[]} onSubmit={onSubmit} />
      </LanguageProviderContext.Provider>
    ),
  }], { initialEntries: ["/tasks/new"] })
  render(<RouterProvider router={router} />)

  const form = screen.getByRole("form")
  fireEvent.change(within(form).getByLabelText(/^タスク名/), { target: { value: "Task" } })
  fireEvent.click(within(form).getByRole("button", { name: "アクションアイテムを追加" }))
  fireEvent.change(within(form).getByLabelText("見積り 1"), { target: { value: "not a duration" } })
  fireEvent.click(within(form).getByRole("button", { name: "アクションアイテム 1 を削除" }))
  fireEvent.click(within(form).getByRole("button", { name: "追加する" }))

  await waitFor(() => expect(onSubmit).toHaveBeenCalledOnce())
  expect(within(form).queryByLabelText("見積り 1")).toBeNull()
})
