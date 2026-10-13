import { expect, test } from "@playwright/test"
import { createAccount, fillCredentials } from "./helpers/auth.ts"

test("configures a weekly allocation, edits its future rule, guards drafts, and edits daily active time", async ({ page, request }) => {
  const credentials = await createAccount(request)
  await page.goto("/login")
  await fillCredentials(page, credentials)
  await page.getByRole("button", { name: "ログイン", exact: true }).click()
  await expect(page).toHaveURL("/today")

  const token = await page.evaluate(() => localStorage.getItem("personal_access_token"))
  expect(token).toBeTruthy()
  const headers = { Authorization: `Bearer ${token}` }
  const userResponse = await page.request.get("/api/users/me", { headers })
  expect(userResponse.status()).toBe(200)
  const user = await userResponse.json() as { timezone: string }
  const today = localDate(new Date(), user.timezone)
  const allocationDate = shiftDate(today, 1)
  const weekday = new Date(`${allocationDate}T12:00:00Z`).getUTCDay()
  const weekStart = mondayOf(allocationDate)

  const projectResponse = await page.request.post("/api/projects", {
    headers,
    data: { title: "Weekly E2E project", goal: "Plan project time", description: "", type: "other", priority: "low", start_date: null, end_date: null },
  })
  expect(projectResponse.status()).toBe(201)
  const project = await projectResponse.json() as { id: string }

  await page.goto("/weekly-plan/edit")
  await expect(page.getByRole("heading", { name: "毎週の割り当てを編集", exact: true })).toBeVisible()
  await page.getByRole("combobox", { name: "プロジェクトを追加", exact: true }).selectOption(project.id)
  const projectForm = page.locator(`#project-${project.id}`)
  await projectForm.getByLabel("週の目標（分）", { exact: true }).fill("600")
  await projectForm.getByLabel("曜日", { exact: true }).selectOption(String(weekday))
  await projectForm.getByLabel("開始", { exact: true }).fill("10:00")
  await projectForm.getByLabel("終了", { exact: true }).fill("11:00")
  const nextDay = projectForm.getByLabel("終了は翌日", { exact: true })
  await nextDay.check()
  await expect(nextDay).toBeChecked()
  await nextDay.uncheck()
  await expect(nextDay).not.toBeChecked()
  await page.getByRole("button", { name: "保存", exact: true }).click()
  await expect(page.getByText("週間プランを保存しました。", { exact: true })).toBeVisible()
  const rulesResponse = await page.request.get("/api/allocation-rules", { headers })
  expect(rulesResponse.status()).toBe(200)
  const storedRules = await rulesResponse.json() as { items: Array<{ project_id: string; start_time: string; end_time: string }> }
  expect(storedRules.items.some((rule) => rule.project_id === project.id && rule.start_time === "10:00" && rule.end_time === "11:00")).toBe(true)
  const plansResponse = await page.request.get("/api/allocation-plans", { headers })
  expect(plansResponse.status()).toBe(200)
  const storedPlans = await plansResponse.json() as { items: Array<{ project_id: string; target_minutes: number }> }
  expect(storedPlans.items.some((plan) => plan.project_id === project.id && plan.target_minutes === 600)).toBe(true)

  await page.goto(`/weekly-plan?week_start=${weekStart}`)
  await expect(page.getByRole("heading", { name: "週間プラン", exact: true })).toBeVisible()
  const scrollport = page.getByTestId("weekly-plan-calendar-scroll")
  const stickyHeader = page.locator(`[data-date="${allocationDate}"] > header`)
  await expect(stickyHeader).toBeVisible()
  await scrollport.evaluate((element) => { element.scrollTop += 300 })
  await expect.poll(async () => {
    const headerBox = await stickyHeader.boundingBox()
    const scrollBox = await scrollport.boundingBox()
    return headerBox && scrollBox ? Math.abs(headerBox.y - scrollBox.y) : 1000
  }).toBeLessThan(3)
  const allocation = page.getByRole("button", { name: /Weekly E2E project, 10:00–11:00/ })
  await expect(allocation).toBeVisible()
  await allocation.click()
  const editor = page.getByRole("dialog", { name: "割り当てを編集" })
  await expect(editor.getByLabel("プロジェクト", { exact: true })).toBeFocused()
  const repeat = editor.getByLabel("毎週繰り返す")
  await expect(repeat).not.toBeChecked()
  await editor.getByLabel("終了時刻", { exact: true }).fill(`${allocationDate}T11:30`)
  await editor.getByRole("button", { name: "閉じる", exact: true }).click()
  const discard = page.getByRole("alertdialog", { name: "変更を破棄しますか？" })
  await expect(discard).toBeVisible()
  await expect(discard.getByRole("button", { name: "編集を続ける", exact: true })).toBeFocused()
  await discard.getByRole("button", { name: "編集を続ける", exact: true }).click()
  await expect(editor.getByLabel("終了時刻", { exact: true })).toHaveValue(`${allocationDate}T11:30`)
  await editor.getByRole("button", { name: "保存", exact: true }).click()
  await expect(page.getByText("週間プランを保存しました。", { exact: true })).toBeVisible()
  const revisedAllocation = page.getByRole("button", { name: /Weekly E2E project, 10:00–11:30/ })
  await expect(revisedAllocation).toBeVisible()
  await revisedAllocation.click()
  const recurrenceEditor = page.getByRole("dialog", { name: "割り当てを編集" })
  const repeatFollowing = recurrenceEditor.getByLabel("毎週繰り返す")
  await expect(repeatFollowing).not.toBeChecked()
  await repeatFollowing.check()
  await recurrenceEditor.getByRole("button", { name: "保存", exact: true }).click()
  await expect(page.getByText("週間プランを保存しました。", { exact: true })).toBeVisible()
  await expect(page.getByRole("button", { name: /Weekly E2E project, 10:00–11:30/ })).toBeVisible()

  const activeProjectsLoaded = page.waitForResponse((response) => new URL(response.url()).pathname === "/api/projects" && response.status() === 200)
  await page.goto("/profile")
  await activeProjectsLoaded
  const activeStart = page.getByLabel("開始時刻", { exact: true })
  await activeStart.fill("10:30")
  await page.locator("form").filter({ hasText: "開始時刻" }).getByRole("button", { name: "設定を保存", exact: true }).click()
  await expect(page.getByText(/Weekly E2E projectの割り当て.*作業可能時間の外にあります。/)).toBeVisible()
  await expect(activeStart).toHaveValue("10:30")

  await page.goto(`/weekly-plan?week_start=${weekStart}`)
  const day = page.locator(`section[data-date="${allocationDate}"]`)
  await day.getByRole("button", { name: /作業可能時間を編集/ }).click()
  const activeEditor = page.getByRole("dialog", { name: `${allocationDate}の作業可能時間` })
  await activeEditor.getByLabel("開始時刻", { exact: true }).fill("08:00")
  await activeEditor.getByLabel("終了時刻", { exact: true }).fill("23:30")
  await activeEditor.getByRole("button", { name: "この日の作業可能時間を保存", exact: true }).click()
  await expect(page.getByText("この日の作業可能時間を保存しました。", { exact: true })).toBeVisible()
  const exceptionResponse = await page.request.get(`/api/allocation-weeks/${weekStart}`, { headers })
  const exceptionWeek = await exceptionResponse.json() as { days: Array<{ date: string; has_active_exception: boolean }> }
  expect(exceptionWeek.days.find((item) => item.date === allocationDate)?.has_active_exception).toBe(true)

  await day.getByRole("button", { name: /作業可能時間を編集/ }).click()
  const resetEditor = page.getByRole("dialog", { name: `${allocationDate}の作業可能時間` })
  await resetEditor.getByRole("button", { name: "標準の作業可能時間に戻す", exact: true }).click()
  await expect(page.getByText("この日の作業可能時間を標準設定に戻しました。", { exact: true })).toBeVisible()
  const resetResponse = await page.request.get(`/api/allocation-weeks/${weekStart}`, { headers })
  const resetWeek = await resetResponse.json() as { days: Array<{ date: string; has_active_exception: boolean }> }
  expect(resetWeek.days.find((item) => item.date === allocationDate)?.has_active_exception).toBe(false)
})

function localDate(date: Date, timezone: string): string {
  const parts = new Intl.DateTimeFormat("en-US", { timeZone: timezone, year: "numeric", month: "2-digit", day: "2-digit" }).formatToParts(date)
  const values = Object.fromEntries(parts.map(({ type, value }) => [type, value]))
  return `${values.year}-${values.month}-${values.day}`
}

function shiftDate(value: string, days: number): string {
  const [year, month, day] = value.split("-").map(Number)
  const date = new Date(Date.UTC(year!, month! - 1, day! + days))
  return `${date.getUTCFullYear()}-${String(date.getUTCMonth() + 1).padStart(2, "0")}-${String(date.getUTCDate()).padStart(2, "0")}`
}

function mondayOf(value: string): string {
  const [year, month, day] = value.split("-").map(Number)
  const date = new Date(Date.UTC(year!, month! - 1, day!))
  date.setUTCDate(date.getUTCDate() - ((date.getUTCDay() + 6) % 7))
  return `${date.getUTCFullYear()}-${String(date.getUTCMonth() + 1).padStart(2, "0")}-${String(date.getUTCDate()).padStart(2, "0")}`
}
