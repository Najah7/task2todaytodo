import { afterEach, expect, test, vi } from "vitest"
import { cleanup, render, screen } from "@testing-library/react"
import { useForm } from "react-hook-form"
import { LanguageProviderContext } from "~/features/i18n/languageContext"
import type { AllocationPlanValues } from "./schema"
import AllocationPlanForm from "."

afterEach(cleanup)

function TestForm() {
  const form = useForm<AllocationPlanValues>({ defaultValues: { projects: [{ projectId: "completed-project", targetMinutes: 60, rules: [] }] } })
  return <LanguageProviderContext.Provider value={{ language: "ja", setLanguage: vi.fn() }}>
    <AllocationPlanForm
      form={form}
      projects={[]}
      projectTitles={{ "completed-project": "Archived research" }}
      timezone="Asia/Tokyo"
      busy={false}
      onSubmit={(event) => event.preventDefault()}
      onCancel={vi.fn()}
      discardConfirmation={{ open: false, onConfirm: vi.fn(), onCancel: vi.fn() }}
    />
  </LanguageProviderContext.Provider>
}

test("shows saved title for a configured completed Project without offering it as an active add option", () => {
  render(<TestForm />)

  expect(screen.getByRole("heading", { name: "Archived research" })).toBeTruthy()
  expect(screen.queryByRole("combobox", { name: "プロジェクトを追加" })).toBeNull()
})
