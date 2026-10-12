import { mkdir } from "node:fs/promises"
import { dirname } from "node:path"
import { expect, test } from "@playwright/test"
import { createAccount, fillCredentials } from "./helpers/auth.ts"

async function confirmTodayRegeneration(page: import("@playwright/test").Page) {
  const dialog = page.getByRole("alertdialog", { name: "今日のリストを更新しますか？" })
  await expect(dialog).toBeVisible()
  await expect(dialog).toContainText("未着手のTask・ActionItemの手動追加や並び順")
  await dialog.getByRole("button", { name: "再生成する", exact: true }).click()
}

test("shows saved timed and all-day schedules across desktop, dark, and mobile layouts", async ({ page, request }, testInfo) => {
  await page.setViewportSize({ width: 1440, height: 1000 })
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
  const date = localDate(new Date(), user.timezone)
  for (const schedule of [
    {
      title: "Design review",
      all_day: false,
      local_start: `${date}T10:00:00`,
      local_end: `${date}T11:00:00`,
      location: "Meeting room",
    },
    {
      title: "Release preparation",
      all_day: true,
      local_start: `${date}T00:00:00`,
      local_end: `${shiftDate(date, 2)}T00:00:00`,
      location: "",
    },
  ]) {
    const response = await page.request.post("/api/schedule-rules", {
      headers,
      data: {
        ...schedule,
        description: "Created by the Schedule visual E2E",
        project_id: "",
        assignee_id: user.user_id,
        timezone: user.timezone,
        frequency: "once",
        interval: 0,
        weekdays: [],
        confirm_todo_regeneration: true,
      },
    })
    expect(response.status()).toBe(201)
  }

  await page.goto(`/schedules?view=week&date=${date}`)
  await expect(page.getByRole("heading", { name: "スケジュール", exact: true })).toBeVisible()
  const selectedDay = page.locator(`section[data-date="${date}"]`)
  await expect(selectedDay.locator('button[title="Design review"]')).toBeVisible()
  await expect(selectedDay.locator('button[title="Release preparation"]')).toBeVisible()
  await saveScreenshot(page, testInfo.outputPath("schedule-week-light.png"))

  const displaySwitch = page.getByRole("switch", { name: "表示モード" })
  await displaySwitch.click()
  await expect(page.locator("html")).toHaveCSS("color-scheme", "dark")
  await saveScreenshot(page, testInfo.outputPath("schedule-week-dark.png"))

  await page.setViewportSize({ width: 390, height: 844 })
  const selectedDayHeader = page.locator(`section[data-date="${date}"] header`)
  await selectedDayHeader.scrollIntoViewIfNeeded()
  await expect(selectedDayHeader).toBeInViewport()
  await expect(selectedDay.locator('button[title="Release preparation"]')).toBeInViewport()
  await saveScreenshot(page, testInfo.outputPath("schedule-week-mobile.png"))

  await page.getByRole("switch", { name: "表示モード" }).click()
  await page.setViewportSize({ width: 1440, height: 1000 })
  await page.getByRole("button", { name: "予定を作成", exact: true }).click()
  await expect(page.getByLabel("タイトル", { exact: true })).toBeVisible()
  await expect(page.getByLabel("開始時刻", { exact: true })).toBeVisible()
  for (const [label, input] of [
    [page.locator('label[for="schedule-start-date"]'), page.locator("#schedule-start-date")],
    [page.locator('label[for="schedule-end-date"]'), page.locator("#schedule-end-date")],
  ]) {
    const labelBox = await label.boundingBox()
    const inputBox = await input.boundingBox()
    expect(labelBox).not.toBeNull()
    expect(inputBox).not.toBeNull()
    expect(Math.abs(labelBox!.y + labelBox!.height / 2 - (inputBox!.y + inputBox!.height / 2))).toBeLessThan(8)
  }
  await saveScreenshot(page, testInfo.outputPath("schedule-create-form.png"))
  await page.getByRole("button", { name: "キャンセル", exact: true }).click()
  await page.locator('button[title="Design review"]').click()
  await expect(page.getByLabel("タイトル", { exact: true })).toHaveValue("Design review")
  await saveScreenshot(page, testInfo.outputPath("schedule-edit-form.png"))
})

test("creates, edits, completes, skips, restores, and deletes a schedule", async ({ page, request }) => {
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
  const date = localDate(new Date(), user.timezone)
  const todayResponse = await page.request.get("/api/today-todo", { headers })
  expect(todayResponse.status()).toBe(200)
  await page.goto(`/schedules?view=week&date=${date}`)

  await page.getByRole("button", { name: "予定を作成", exact: true }).click()
  await expect(page.getByRole("heading", { name: "予定を作成", exact: true })).toBeVisible()
  await page.getByLabel("タイトル", { exact: true }).fill("UI schedule")
  await page.getByLabel("開始日", { exact: true }).fill(date)
  await page.getByLabel("終了日", { exact: true }).fill(date)
  await page.getByLabel("開始時刻", { exact: true }).fill("10:00")
  await page.getByLabel("終了時刻", { exact: true }).fill("11:00")
  await page.getByRole("button", { name: "作成する", exact: true }).click()
  await confirmTodayRegeneration(page)
  await expect(page.getByText("予定を作成しました。", { exact: true })).toBeVisible()
  const scheduleButton = page.locator('button[title="UI schedule"]').first()
  await expect(scheduleButton).toBeVisible()

  await scheduleButton.click()
  await expect(page).toHaveURL(/\/schedules\/.*\/\d{4}-\d{2}-\d{2}\/edit/)
  await page.getByLabel("タイトル", { exact: true }).fill("UI schedule revised")
  await page.getByRole("button", { name: "保存する", exact: true }).click()
  await expect(page.getByText("予定を保存しました。", { exact: true })).toBeVisible()
  await expect(page.locator('button[title="UI schedule revised"]')).toBeVisible()

  const operations = page.locator('summary[aria-label="予定を編集: UI schedule revised"]').first()
  await operations.click()
  await page.getByRole("button", { name: "完了にする", exact: true }).click()
  await expect(page.getByText("予定を完了しました。", { exact: true })).toBeVisible()
  await operations.click()
  await page.getByRole("button", { name: "未完了に戻す", exact: true }).click()
  await expect(page.getByText("予定を未完了に戻しました。", { exact: true })).toBeVisible()

  await page.getByText("スキップした予定を表示", { exact: true }).click()
  await operations.click()
  await page.getByRole("button", { name: "スキップ", exact: true }).click()
  await confirmTodayRegeneration(page)
  await expect(page.getByText("予定をスキップしました。", { exact: true })).toBeVisible()
  await operations.click()
  await page.getByRole("button", { name: "スキップを解除", exact: true }).click()
  await confirmTodayRegeneration(page)
  await expect(page.getByText("スキップを解除しました。", { exact: true })).toBeVisible()

  await operations.click()
  await page.getByRole("button", { name: "この予定だけを削除", exact: true }).click()
  const dialog = page.getByRole("alertdialog", { name: "予定を削除しますか？" })
  await dialog.getByRole("button", { name: "削除する", exact: true }).click()
  await confirmTodayRegeneration(page)
  await expect(page.getByText("予定を削除しました。", { exact: true })).toBeVisible()
  await expect(page.locator('button[title="UI schedule revised"]')).toHaveCount(0)

  await page.getByRole("button", { name: "予定を作成", exact: true }).click()
  await page.getByLabel("タイトル", { exact: true }).fill("UI all-day")
  await page.getByLabel("開始日", { exact: true }).fill(date)
  await page.getByLabel("終了日", { exact: true }).fill(date)
  await page.getByLabel("終日", { exact: true }).check()
  await page.getByRole("button", { name: "作成する", exact: true }).click()
  await confirmTodayRegeneration(page)
  await expect(page.locator('button[title="UI all-day"]')).toBeVisible()
})

test("edits and deletes a recurring schedule from the selected occurrence onward", async ({ page, request }) => {
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
  const date = shiftDate(localDate(new Date(), user.timezone), 1)
  const response = await page.request.post("/api/schedule-rules", {
    headers,
    data: {
      title: "Recurring agenda",
      description: "Recurring E2E fixture",
      location: "",
      project_id: "",
      assignee_id: user.user_id,
      timezone: user.timezone,
      all_day: false,
      local_start: `${date}T10:00:00`,
      local_end: `${date}T11:00:00`,
      frequency: "daily",
      interval: 1,
      weekdays: [],
      end_date: shiftDate(date, 5),
      confirm_todo_regeneration: true,
    },
  })
  expect(response.status()).toBe(201)

  await page.goto(`/schedules?view=week&date=${date}`)
  const dayView = page.getByRole("button", { name: "日", exact: true })
  await dayView.click()
  await expect(page).toHaveURL((url) => url.searchParams.get("date") === date && url.searchParams.get("view") === "day")
  await expect(dayView).toHaveAttribute("aria-pressed", "true")
  const selectedDay = page.locator(`section[data-date="${date}"]`)
  await expect(page.locator("section[data-date]")).toHaveCount(1)
  const nextDay = page.locator(`section[data-date="${shiftDate(date, 1)}"]`)
  await selectedDay.locator('button[title="Recurring agenda"]').click()
  await expect(page).toHaveURL(/\/schedules\/.*\/\d{4}-\d{2}-\d{2}\/edit/)
  await expect(page.getByLabel("変更範囲", { exact: true })).toBeVisible()
  await page.getByLabel("変更範囲", { exact: true }).selectOption("following")
  await page.getByLabel("タイトル", { exact: true }).fill("Recurring agenda updated")
  await page.getByRole("button", { name: "保存する", exact: true }).click()
  await expect(page.getByText("予定を保存しました。", { exact: true })).toBeVisible()
  await expect(page.locator('button[title="Recurring agenda updated"]')).toBeVisible()

  await selectedDay.locator('button[title="Recurring agenda updated"]').click()
  await expect(page.getByLabel("変更範囲", { exact: true })).toHaveValue("current")
  await page.getByLabel("タイトル", { exact: true }).fill("Current agenda only")
  await page.getByRole("button", { name: "保存する", exact: true }).click()
  await expect(page.getByText("予定を保存しました。", { exact: true })).toBeVisible()
  await expect(selectedDay.locator('button[title="Current agenda only"]')).toBeVisible()

  await page.getByRole("button", { name: "次へ", exact: true }).click()
  await expect(page).toHaveURL((url) => url.searchParams.get("date") === shiftDate(date, 1) && url.searchParams.get("view") === "day")
  await expect(page.locator("section[data-date]")).toHaveCount(1)
  await expect(nextDay.locator('button[title="Recurring agenda updated"]')).toBeVisible()
  const editMenu = nextDay.locator('summary[aria-label="予定を編集: Recurring agenda updated"]')
  await editMenu.click()
  await page.getByRole("button", { name: "この予定以降を削除", exact: true }).click()
  const deleteDialog = page.getByRole("alertdialog", { name: "予定を削除しますか？" })
  await expect(deleteDialog.getByText(/以降の繰り返し予定/)).toBeVisible()
  await deleteDialog.getByRole("button", { name: "削除する", exact: true }).click()
  await expect(page.getByText("予定を削除しました。", { exact: true })).toBeVisible()
  await expect(nextDay.locator('button[title="Recurring agenda updated"]')).toHaveCount(0)
  await page.getByRole("button", { name: "前へ", exact: true }).click()
  await expect(page).toHaveURL((url) => url.searchParams.get("date") === date && url.searchParams.get("view") === "day")
  await expect(selectedDay.locator('button[title="Current agenda only"]')).toBeVisible()
})

test("filters Project schedules and opens the correct day from month overflow", async ({ page, request }) => {
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
  const date = localDate(new Date(), user.timezone)
  const projectResponse = await page.request.post("/api/projects", {
    headers,
    data: {
      title: "Schedule project",
      goal: "Group project schedules",
      description: "Created by the Schedule E2E",
      type: "other",
      priority: "low",
      start_date: null,
      end_date: null,
    },
  })
  expect(projectResponse.status()).toBe(201)
  const project = await projectResponse.json() as { id: string }

  for (let index = 1; index <= 4; index++) {
    const response = await page.request.post("/api/schedule-rules", {
      headers,
      data: {
        title: `Project review ${index}`,
        description: "Month overflow fixture",
        location: "",
        project_id: project.id,
        assignee_id: user.user_id,
        timezone: user.timezone,
        all_day: false,
        local_start: `${date}T${String(9 + index).padStart(2, "0")}:00:00`,
        local_end: `${date}T${String(10 + index).padStart(2, "0")}:00:00`,
        frequency: "once",
        interval: 0,
        weekdays: [],
        confirm_todo_regeneration: true,
      },
    })
    expect(response.status()).toBe(201)
  }

  await page.goto(`/schedules?view=month&date=${date}`)
  await page.getByLabel("プロジェクトで絞り込み", { exact: true }).selectOption(project.id)
  await expect(page.locator('button[title="Project review 1"]')).toBeVisible()
  const moreEvents = page.getByRole("button", { name: /\+1/ }).first()
  await expect(moreEvents).toBeVisible()
  await moreEvents.click()
  await expect(page).toHaveURL((url) => url.searchParams.get("date") === date && url.searchParams.get("view") === "day")
  await expect(page).toHaveURL(new RegExp(`[?&]project_id=${project.id}`))
})

async function saveScreenshot(page: import("@playwright/test").Page, path: string) {
  await mkdir(dirname(path), { recursive: true })
  await page.screenshot({ path, fullPage: true })
}

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
