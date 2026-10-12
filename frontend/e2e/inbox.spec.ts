import { mkdir } from "node:fs/promises"
import { dirname } from "node:path"
import { expect, test } from "@playwright/test"
import { createAccount, fillCredentials } from "./helpers/auth.ts"

test("quick-creates a Task and multi-day all-day Schedule, filters, edits, and completes both", async ({ page, request }, testInfo) => {
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
  const user = await userResponse.json() as { user_id: string; timezone: string }
  const today = localDate(new Date(), user.timezone)
  const tomorrow = shiftDate(today, 1)
  const projectResponse = await page.request.post("/api/projects", {
    headers,
    data: {
      title: "Inbox E2E project",
      goal: "Group quick-created work",
      description: "",
      type: "other",
      priority: "low",
      start_date: null,
      end_date: null,
    },
  })
  expect(projectResponse.status()).toBe(201)
  const project = await projectResponse.json() as { id: string }

  await page.goto("/inbox")
  await expect(page.getByRole("heading", { name: "インボックス", exact: true })).toBeVisible()
  const quickCreate = page.getByRole("form", { name: "すぐに追加" })
  const taskTitle = "Inbox quick Task"
  await quickCreate.getByLabel("タスク名", { exact: true }).fill(taskTitle)
  await quickCreate.getByRole("combobox", { name: "プロジェクト", exact: true }).selectOption(project.id)
  await quickCreate.getByRole("button", { name: "追加", exact: true }).click()
  const taskRow = page.getByRole("listitem").filter({ hasText: taskTitle })
  await expect(taskRow).toBeVisible()
  await expect(taskRow.getByRole("button", { name: "今日に追加", exact: true })).toBeDisabled()

  const scheduleTitle = "Inbox multi-day all-day Schedule"
  await quickCreate.getByRole("tab", { name: "予定", exact: true }).click()
  await quickCreate.getByLabel("予定名", { exact: true }).fill(scheduleTitle)
  await quickCreate.getByRole("combobox", { name: "プロジェクト", exact: true }).selectOption(project.id)
  await quickCreate.getByLabel("開始日", { exact: true }).fill(today)
  await quickCreate.getByLabel("終了日", { exact: true }).fill(tomorrow)
  await quickCreate.getByLabel("終日", { exact: true }).check()
  await saveScreenshot(page, testInfo.outputPath("inbox-quick-create-schedule.png"))
  await quickCreate.getByRole("button", { name: "追加", exact: true }).click()
  const scheduleRow = page.getByRole("listitem").filter({ hasText: scheduleTitle })
  await expect(scheduleRow).toBeVisible()
  await expect(scheduleRow).toContainText("終日")
  await saveScreenshot(page, testInfo.outputPath("inbox-mixed-light.png"))
  const displaySwitch = page.getByRole("switch", { name: "表示モード" })
  await displaySwitch.click()
  await expect(page.locator("html")).toHaveCSS("color-scheme", "dark")
  await saveScreenshot(page, testInfo.outputPath("inbox-mixed-dark.png"))
  await displaySwitch.click()
  await page.setViewportSize({ width: 390, height: 844 })
  await saveScreenshot(page, testInfo.outputPath("inbox-mixed-mobile.png"))
  await page.setViewportSize({ width: 1440, height: 1000 })

  const inboxResponse = await page.request.get("/api/inbox", { headers })
  expect(inboxResponse.status()).toBe(200)
  const inbox = await inboxResponse.json() as {
    items: Array<{
      type: string
      title: string
      project_id: string | null
      task: { id: string; revision: number } | null
      schedule: { rule_id: string; occurrence_date: string; all_day: boolean; start_at: string; end_at: string; revision: number } | null
    }>
  }
  const createdTask = inbox.items.find((item) => item.title === taskTitle)
  const createdSchedule = inbox.items.find((item) => item.title === scheduleTitle)
  expect(createdTask).toMatchObject({ type: "task", project_id: project.id })
  expect(createdTask?.task?.revision).toBeGreaterThan(0)
  expect(createdSchedule).toMatchObject({ type: "schedule", project_id: project.id })
  expect(createdSchedule?.schedule).toMatchObject({
    all_day: true,
    occurrence_date: today,
  })
  expect(localDate(new Date(createdSchedule!.schedule!.start_at), user.timezone)).toBe(today)
  expect(localDate(new Date(createdSchedule!.schedule!.end_at), user.timezone)).toBe(shiftDate(tomorrow, 1))

  const filterTabs = page.getByRole("tablist", { name: "インボックス" })
  await filterTabs.getByRole("tab", { name: /^予定/ }).click()
  const filters = page.getByRole("region", { name: "インボックスの一覧", exact: true })
  await filters.getByRole("combobox", { name: "プロジェクト", exact: true }).selectOption(project.id)
  await filters.getByLabel("タイトルを検索", { exact: true }).fill("MULTI-DAY")
  await expect(page).toHaveURL((url) =>
    url.pathname === "/inbox"
    && url.searchParams.get("type") === "schedule"
    && url.searchParams.get("project_id") === project.id
    && url.searchParams.get("title") === "MULTI-DAY")
  await expect(scheduleRow).toBeVisible()
  await expect(taskRow).toHaveCount(0)

  await filters.getByLabel("タイトルを検索", { exact: true }).fill("")
  await filterTabs.getByRole("tab", { name: /^タスク/ }).click()
  await expect(taskRow).toBeVisible()
  await expect(page).toHaveURL((url) => url.searchParams.get("type") === "task" && url.searchParams.get("project_id") === project.id)

  await taskRow.getByRole("button", { name: /編集/ }).click()
  await expect(page).toHaveURL(/\/tasks\/[^/]+\/edit$/)
  const revisedTaskTitle = "Inbox quick Task revised"
  await page.getByLabel("タスク名 必須", { exact: true }).fill(revisedTaskTitle)
  await page.getByRole("button", { name: "変更を保存", exact: true }).click()
  await expect(page).toHaveURL((url) =>
    url.pathname === "/inbox" && url.searchParams.get("type") === "task" && url.searchParams.get("project_id") === project.id)
  const revisedTaskRow = page.getByRole("listitem").filter({ hasText: revisedTaskTitle })
  await expect(revisedTaskRow).toBeVisible()
  await revisedTaskRow.getByRole("button", { name: /完了/ }).click()
  await expect(revisedTaskRow).toHaveCount(0)

  await filterTabs.getByRole("tab", { name: /^予定/ }).click()
  const activeScheduleRow = page.getByRole("listitem").filter({ hasText: scheduleTitle })
  await expect(activeScheduleRow).toBeVisible()
  await activeScheduleRow.getByRole("button", { name: /完了/ }).click()
  await expect(activeScheduleRow).toHaveCount(0)
  const occurrenceResponse = await page.request.get(
    `/api/schedules/${createdSchedule?.schedule?.rule_id}/${createdSchedule?.schedule?.occurrence_date}`,
    { headers },
  )
  expect(occurrenceResponse.status()).toBe(200)
  expect(await occurrenceResponse.json()).toMatchObject({ completed: true, all_day: true })
})

test("shows a virtual recurring Schedule, edits that occurrence, and completes it", async ({ page, request }) => {
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
  const user = await userResponse.json() as { user_id: string; timezone: string }
  const today = localDate(new Date(), user.timezone)
  const tomorrow = shiftDate(today, 1)
  const recurringTitle = "Virtual Inbox occurrence"
  const create = await page.request.post("/api/schedule-rules", {
    headers,
    data: {
      title: recurringTitle,
      description: "Created through the real API for Inbox E2E",
      location: "",
      project_id: "",
      assignee_id: user.user_id,
      timezone: user.timezone,
      all_day: false,
      local_start: `${today}T10:00:00`,
      local_end: `${today}T11:00:00`,
      frequency: "daily",
      interval: 1,
      weekdays: [],
    },
  })
  expect(create.status()).toBe(201)
  const rule = await create.json() as { id: string }

  const initialInbox = await page.request.get("/api/inbox?type=schedule", { headers })
  expect(initialInbox.status()).toBe(200)
  const initialItems = (await initialInbox.json()).items as Array<{
    title: string
    schedule: { rule_id: string; occurrence_date: string; occurrence_id: string | null; revision: number } | null
  }>
  const virtual = initialItems.find((item) =>
    item.schedule?.rule_id === rule.id && item.schedule.occurrence_date === tomorrow)
  expect(virtual?.schedule).toMatchObject({ occurrence_id: null, occurrence_date: tomorrow })

  await page.goto("/inbox?type=schedule")
  await expect(page.getByRole("heading", { name: "インボックス", exact: true })).toBeVisible()
  const recurringRows = page.getByRole("listitem").filter({ hasText: recurringTitle })
  await expect(recurringRows).toHaveCount(7)
  const virtualRow = recurringRows.nth(1)
  await expect(virtualRow).toBeVisible()
  await virtualRow.getByRole("button", { name: /編集/ }).click()
  await expect(page).toHaveURL(new RegExp(`/schedules/${rule.id}/${tomorrow}/edit`))
  await page.getByLabel("タイトル", { exact: true }).fill("Edited virtual occurrence")
  await page.getByRole("button", { name: "保存する", exact: true }).click()
  await expect(page).toHaveURL((url) => url.pathname === "/inbox" && url.searchParams.get("type") === "schedule")
  const editedRow = page.getByRole("listitem").filter({ hasText: "Edited virtual occurrence" })
  await expect(editedRow).toBeVisible()

  const afterEdit = await page.request.get("/api/inbox?type=schedule", { headers })
  expect(afterEdit.status()).toBe(200)
  const editedItems = (await afterEdit.json()).items as Array<{
    title: string
    schedule: { rule_id: string; occurrence_date: string; occurrence_id: string | null; revision: number } | null
  }>
  const editedOccurrence = editedItems.find((item) =>
    item.schedule?.rule_id === rule.id && item.schedule.occurrence_date === tomorrow)
  expect(editedOccurrence).toMatchObject({ title: "Edited virtual occurrence" })
  expect(editedOccurrence?.schedule?.occurrence_id).toBeTruthy()
  expect(editedOccurrence?.schedule?.revision).toBeGreaterThan(0)

  await editedRow.getByRole("button", { name: /完了/ }).click()
  await expect(editedRow).toHaveCount(0)
  const completed = await page.request.get(`/api/schedules/${rule.id}/${tomorrow}`, { headers })
  expect(completed.status()).toBe(200)
  expect(await completed.json()).toMatchObject({ completed: true, title: "Edited virtual occurrence" })
})

function localDate(date: Date, timezone: string): string {
  const parts = new Intl.DateTimeFormat("en-US", {
    timeZone: timezone,
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
  }).formatToParts(date)
  const values = Object.fromEntries(parts.map(({ type, value }) => [type, value]))
  return `${values.year}-${values.month}-${values.day}`
}

function shiftDate(value: string, days: number): string {
  const [year, month, day] = value.split("-").map(Number)
  const date = new Date(Date.UTC(year!, month! - 1, day! + days))
  return `${date.getUTCFullYear()}-${String(date.getUTCMonth() + 1).padStart(2, "0")}-${String(date.getUTCDate()).padStart(2, "0")}`
}

async function saveScreenshot(page: import("@playwright/test").Page, path: string) {
  await mkdir(dirname(path), { recursive: true })
  await page.screenshot({ path, fullPage: true })
}
