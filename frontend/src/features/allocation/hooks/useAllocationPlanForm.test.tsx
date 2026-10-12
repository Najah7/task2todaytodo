import { afterEach, expect, test, vi } from "vitest"
import { act, cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react"
import { createMemoryRouter, RouterProvider } from "react-router"
import { LanguageProviderContext } from "~/features/i18n/languageContext"
import { AllocationAPIDataContext, type AllocationAPIData } from "~/features/allocation/providers/AllocationAPIDataProvider/context"
import { AllocationActionContext, type AllocationActions } from "~/features/allocation/providers/AllocationActionProvider/context"
import { AllocationNavigationContext } from "~/features/allocation/providers/AllocationNavigationProvider/context"
import type { RestAllocationPlansResponse, RestAllocationRulesResponse } from "~/api/generated/allocations"
import type { RestProjectResponse } from "~/api/generated/projects"
import { useAllocationPlanForm } from "./useAllocationPlanForm"
import AllocationPlanForm from "~/features/allocation/components/AllocationPlanForm"

afterEach(() => cleanup())

const initialRules: RestAllocationRulesResponse = {
  revision: "rules-v1",
  timezone: "Asia/Tokyo",
  items: [{ id: "rule-1", revision: 1, project_id: "project-1", project_title: "Project One", weekday: 1, start_time: "09:00", end_time: "10:00", end_day_offset: 0, timezone: "Asia/Tokyo" }],
}
const initialPlans: RestAllocationPlansResponse = {
  revision: "plans-v1",
  items: [{ project_id: "project-1", title: "Project One", target_minutes: 60 }],
}
const projects: RestProjectResponse[] = [{ id: "project-1", title: "Project One" } as RestProjectResponse]

function apiData(rules = initialRules, plans = initialPlans): AllocationAPIData {
  return { state: "ready", weekStart: "2026-10-12", timezone: "Asia/Tokyo", rules, plans, projects } as AllocationAPIData
}

function Editor() {
  const result = useAllocationPlanForm()
  return <AllocationPlanForm form={result.form} {...result.viewProps} />
}

function mountForm(savePlanParts: AllocationActions["savePlanParts"], rules = initialRules, plans = initialPlans) {
  const backToPlan = vi.fn()
  const navigation = { weekStart: "2026-10-12", changeWeek: vi.fn(), editPlan: vi.fn(), backToPlan }
  const action = { busy: false, savePlanParts } as AllocationActions
  const router = createMemoryRouter([{ path: "/", element: <Editor /> }], { initialEntries: ["/"] })
  const tree = (nextRules: RestAllocationRulesResponse, nextPlans: RestAllocationPlansResponse) =>
    <LanguageProviderContext.Provider value={{ language: "ja", setLanguage: vi.fn() }}>
      <AllocationAPIDataContext.Provider value={apiData(nextRules, nextPlans)}>
        <AllocationActionContext.Provider value={action}>
          <AllocationNavigationContext.Provider value={navigation}><RouterProvider router={router} /></AllocationNavigationContext.Provider>
        </AllocationActionContext.Provider>
      </AllocationAPIDataContext.Provider>
    </LanguageProviderContext.Provider>
  const view = render(tree(rules, plans))
  return { ...view, backToPlan, rerenderData: (nextRules: RestAllocationRulesResponse, nextPlans: RestAllocationPlansResponse) => view.rerender(tree(nextRules, nextPlans)) }
}

function save() {
  fireEvent.click(screen.getByRole("button", { name: "保存" }))
}

test("keeps successful rules after target failure, retries only targets, and reuses newly returned rule identity", async () => {
  const savedRules: RestAllocationRulesResponse = {
    revision: "rules-v2",
    timezone: "Asia/Tokyo",
    items: [
      initialRules.items![0]!,
      { id: "new-rule", revision: 1, project_id: "project-1", project_title: "Project One", weekday: 1, start_time: "11:00", end_time: "12:00", end_day_offset: 0, timezone: "Asia/Tokyo" },
    ],
  }
  const savePlanParts = vi.fn()
    .mockResolvedValueOnce({ rules: savedRules, errors: { plans: new Error("target write failed") } })
    .mockResolvedValueOnce({ plans: { revision: "plans-v2", items: [{ project_id: "project-1", title: "Project One", target_minutes: 90 }] }, errors: {} })
    .mockResolvedValueOnce({ rules: { ...savedRules, revision: "rules-v3", items: savedRules.items!.map((rule) => rule.id === "new-rule" ? { ...rule, revision: 2, start_time: "11:30" } : rule) }, errors: {} })
  mountForm(savePlanParts)

  fireEvent.click(screen.getByRole("button", { name: "曜日と時間帯を追加" }))
  const times = screen.getAllByLabelText("開始")
  fireEvent.change(times[1]!, { target: { value: "11:00" } })
  fireEvent.change(screen.getAllByLabelText("終了")[1]!, { target: { value: "12:00" } })
  fireEvent.change(screen.getByLabelText("週の目標（分）"), { target: { value: "90" } })
  await act(async () => save())

  expect((await screen.findByRole("status")).textContent).toContain("割り当てルールは保存されました。")
  expect(screen.getByRole("alert").textContent).toContain("週の目標を保存できませんでした")
  expect(savePlanParts).toHaveBeenNthCalledWith(1, expect.objectContaining({ rules: expect.any(Object), plans: expect.any(Object), revisions: { rules: "rules-v1", plans: "plans-v1" } }))

  await act(async () => save())
  expect(savePlanParts.mock.calls[1]![0].rules).toBeUndefined()
  expect(savePlanParts.mock.calls[1]![0].plans).toBeDefined()

  fireEvent.change(screen.getAllByLabelText("開始")[1]!, { target: { value: "11:30" } })
  await act(async () => save())
  expect(savePlanParts.mock.calls[2]![0].plans).toBeUndefined()
  expect(savePlanParts.mock.calls[2]![0].rules?.items).toEqual(expect.arrayContaining([expect.objectContaining({ id: "new-rule", revision: 1, start_time: "11:30" })]))
})

test("keeps successful targets after rule failure and retries only rules", async () => {
  const savePlanParts = vi.fn()
    .mockResolvedValueOnce({ plans: initialPlans, errors: { rules: new Error("rule write failed") } })
    .mockResolvedValueOnce({ rules: initialRules, errors: {} })
  mountForm(savePlanParts)

  fireEvent.change(screen.getByLabelText("週の目標（分）"), { target: { value: "120" } })
  fireEvent.change(screen.getByLabelText("開始"), { target: { value: "09:30" } })
  await act(async () => save())
  expect((await screen.findByRole("alert")).textContent).toContain("割り当てルールを保存できませんでした")

  await act(async () => save())
  expect(savePlanParts.mock.calls[1]![0].rules).toBeDefined()
  expect(savePlanParts.mock.calls[1]![0].plans).toBeUndefined()
})

test("uses token and draft values from first ready snapshot after background data changes", async () => {
  const savePlanParts = vi.fn().mockResolvedValue({ rules: initialRules, errors: {} })
  const view = mountForm(savePlanParts)
  const refreshedRules = { ...initialRules, revision: "rules-v2", items: initialRules.items!.map((rule) => ({ ...rule, revision: 8, start_time: "12:00" })) }
  const refreshedPlans = { ...initialPlans, revision: "plans-v2", items: initialPlans.items!.map((plan) => ({ ...plan, target_minutes: 200 })) }
  view.rerenderData(refreshedRules, refreshedPlans)

  expect((screen.getByLabelText("週の目標（分）") as HTMLInputElement).value).toBe("60")
  expect((screen.getByLabelText("開始") as HTMLInputElement).value).toBe("09:00")
  fireEvent.change(screen.getByLabelText("開始"), { target: { value: "09:15" } })
  await act(async () => save())
  await waitFor(() => expect(savePlanParts).toHaveBeenCalledTimes(1))
  expect(savePlanParts).toHaveBeenCalledWith(expect.objectContaining({ revisions: { rules: "rules-v1", plans: "plans-v1" } }))
})

