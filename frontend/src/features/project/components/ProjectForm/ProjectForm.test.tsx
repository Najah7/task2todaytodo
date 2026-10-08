import { cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react"
import type { ComponentProps } from "react"
import { afterEach, expect, test, vi } from "vitest"
import { createMemoryRouter, RouterProvider } from "react-router"
import { LanguageProviderContext } from "~/features/i18n/languageContext"
import ProjectForm, { type ProjectFormOption } from "."
import { emptyProjectFormValues, type ProjectFormValues } from "./schema"

const types: ProjectFormOption[] = [{ value: "other", label: "Other" }]
const priorities: ProjectFormOption[] = [{ value: "low", label: "Low" }]

afterEach(cleanup)

function renderForm(overrides: Partial<ComponentProps<typeof ProjectForm>> = {}) {
  const props: ComponentProps<typeof ProjectForm> = {
    mode: "create",
    heading: "projects.form.createTitle",
    initialValues: emptyProjectFormValues,
    types,
    priorities,
    onSubmit: vi.fn().mockResolvedValue({ saved: true, revision: 2 }),
    ...overrides,
  }
  const router = createMemoryRouter([{ path: "*", element: <ProjectForm {...props} /> }], { initialEntries: ["/projects/new"] })
  return render(
    <LanguageProviderContext.Provider value={{ language: "ja", setLanguage: vi.fn() }}>
      <RouterProvider router={router} />
    </LanguageProviderContext.Provider>,
  )
}

test("shows required title and date order validation beside their fields", async () => {
  renderForm()
  fireEvent.click(screen.getByRole("button", { name: /作成する|Create/ }))
  expect(await screen.findByText("プロジェクト名を入力してください。")).toBeTruthy()

  fireEvent.change(screen.getByLabelText(/プロジェクト名|Project name/), { target: { value: "Quarterly report" } })
  fireEvent.change(screen.getByLabelText(/開始日|Start date/), { target: { value: "2026-10-20" } })
  fireEvent.change(screen.getByLabelText(/期限|Deadline/), { target: { value: "2026-10-10" } })
  fireEvent.click(screen.getByRole("button", { name: /作成する|Create/ }))
  expect(await screen.findByText("期限は開始日以降の日付を入力してください。")).toBeTruthy()
  expect(screen.getByLabelText(/期限|Deadline/).getAttribute("aria-invalid")).toBe("true")
})

test("preserves a dirty draft until the user confirms leaving", async () => {
  renderForm()
  fireEvent.change(screen.getByLabelText(/プロジェクト名|Project name/), { target: { value: "A draft" } })
  fireEvent.click(screen.getByRole("button", { name: /キャンセル|Cancel/ }))

  const dialog = await screen.findByRole("alertdialog", { name: /変更を破棄|Discard changes/ })
  expect(dialog).toBeTruthy()
  fireEvent.click(screen.getByRole("button", { name: /編集を続ける|Keep editing/ }))
  await waitFor(() => expect(screen.queryByRole("alertdialog")).toBeNull())
  expect((screen.getByLabelText(/プロジェクト名|Project name/) as HTMLInputElement).value).toBe("A draft")
})

test("requires explicit discard confirmation before loading the current revision", async () => {
  const latest: ProjectFormValues = {
    title: "Latest",
    goal: "",
    description: "",
    type: "other",
    priority: "low",
    startDate: "",
    endDate: "",
  }
  const onLoadLatest = vi.fn().mockResolvedValue({ values: latest, revision: 8 })
  const onConflictResolved = vi.fn()
  renderForm({ mode: "edit", heading: "projects.form.editTitle", hasConflict: true, onLoadLatest, onConflictResolved })
  fireEvent.change(screen.getByLabelText(/プロジェクト名|Project name/), { target: { value: "My draft" } })
  fireEvent.click(screen.getByRole("button", { name: /最新の内容を読み込む|Load latest content/ }))
  expect(await screen.findByRole("alertdialog", { name: /最新の内容を読み込みますか|Load the latest content/ })).toBeTruthy()
  expect((screen.getByLabelText(/プロジェクト名|Project name/) as HTMLInputElement).value).toBe("My draft")
  const reloadDialog = screen.getByRole("alertdialog", { name: /最新の内容を読み込みますか|Load the latest content/ })
  fireEvent.click(within(reloadDialog).getByRole("button", { name: /最新の内容を読み込む|Load latest content/ }))
  await waitFor(() => expect((screen.getByLabelText(/プロジェクト名|Project name/) as HTMLInputElement).value).toBe("Latest"))
  expect(onLoadLatest).toHaveBeenCalledOnce()
  expect(onConflictResolved).toHaveBeenCalledWith(8)
})

test("shows a server start-date error beside its input", async () => {
  renderForm({
    onSubmit: vi.fn().mockResolvedValue({
      saved: false,
      fieldErrors: { startDate: "projects.form.dateOrder" },
    }),
  })
  fireEvent.change(screen.getByLabelText(/プロジェクト名|Project name/), { target: { value: "A project" } })
  fireEvent.click(screen.getByRole("button", { name: /作成する|Create/ }))

  const startDate = screen.getByLabelText(/開始日|Start date/)
  expect(await screen.findByText("期限は開始日以降の日付を入力してください。")).toBeTruthy()
  expect(startDate.getAttribute("aria-invalid")).toBe("true")
  expect(startDate.getAttribute("aria-describedby")).toBe("project-start-date-error")
})

test("keeps the draft when loading latest revision fails", async () => {
  const onError = vi.fn()
  const onLoadLatest = vi.fn().mockRejectedValue(new Error("network failure"))
  renderForm({ mode: "edit", heading: "projects.form.editTitle", hasConflict: true, onLoadLatest, onError })
  fireEvent.change(screen.getByLabelText(/プロジェクト名|Project name/), { target: { value: "Keep this draft" } })
  fireEvent.click(screen.getByRole("button", { name: /最新の内容を読み込む|Load latest content/ }))
  const reloadDialog = await screen.findByRole("alertdialog", { name: /最新の内容を読み込みますか|Load the latest content/ })
  fireEvent.click(within(reloadDialog).getByRole("button", { name: /最新の内容を読み込む|Load latest content/ }))

  await waitFor(() => expect(onError).toHaveBeenCalledOnce())
  expect((screen.getByLabelText(/プロジェクト名|Project name/) as HTMLInputElement).value).toBe("Keep this draft")
  expect(screen.getByRole("alertdialog")).toBeTruthy()
})

test("allows the successful save navigation after it clears the dirty draft", async () => {
  let router: ReturnType<typeof createMemoryRouter>
  const form = (
    <ProjectForm
      mode="create"
      heading="projects.form.createTitle"
      initialValues={emptyProjectFormValues}
      types={types}
      priorities={priorities}
      onSubmit={async () => ({ saved: true, revision: 2 })}
      onSaveComplete={() => router.navigate("/projects?status=open")}
    />
  )
  router = createMemoryRouter([
    { path: "/projects/new", element: form },
    { path: "/projects", element: <h1>Projects open</h1> },
  ], { initialEntries: ["/projects/new"] })
  render(
    <LanguageProviderContext.Provider value={{ language: "ja", setLanguage: vi.fn() }}>
      <RouterProvider router={router} />
    </LanguageProviderContext.Provider>,
  )

  fireEvent.change(screen.getByLabelText(/プロジェクト名|Project name/), { target: { value: "A saved project" } })
  fireEvent.click(screen.getByRole("button", { name: /作成する|Create/ }))
  expect(await screen.findByRole("heading", { name: "Projects open" })).toBeTruthy()
  expect(screen.queryByRole("alertdialog")).toBeNull()
})
