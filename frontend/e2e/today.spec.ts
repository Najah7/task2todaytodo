import { expect, test, type APIRequestContext, type Page } from "@playwright/test"
import { createAccount, fillCredentials } from "./helpers/auth.ts"

async function signIn(page: Page, request: APIRequestContext) {
  const credentials = await createAccount(request)
  await page.goto("/login")
  await fillCredentials(page, credentials)
  const todayResponse = page.waitForResponse((response) => new URL(response.url()).pathname === "/api/today-todo" && response.request().method() === "GET")
  await page.getByRole("button", { name: "ログイン", exact: true }).click()
  await expect(page).toHaveURL("/today")
  expect((await todayResponse).status()).toBe(200)
  const token = await page.evaluate(() => localStorage.getItem("personal_access_token"))
  expect(token).toBeTruthy()
  return { headers: { Authorization: `Bearer ${token}` } }
}

async function createTaskActionItem(request: APIRequestContext, headers: Record<string, string>, title: string, estimatedMinutes = 30, projectId?: string) {
  const taskRequest = {
    title, description: "", priority: "high", due_date: null, manual_estimated_minutes: estimatedMinutes,
  }
  const taskResponse = await request.post(projectId ? `/api/projects/${projectId}/tasks` : "/api/tasks", {
    headers,
    data: taskRequest,
  })
  expect(taskResponse.status()).toBe(201)
  const task = await taskResponse.json() as { id: string }
  const actionItemResponse = await request.post(`/api/tasks/${task.id}/action-items`, {
    headers,
    data: { title: `${title} action`, description: "", estimated_minutes: estimatedMinutes, priority: "medium" },
  })
  expect(actionItemResponse.status()).toBe(201)
  return { task: task as { id: string }, actionItem: await actionItemResponse.json() as { id: string } }
}

async function addTaskChildToToday(page: Page, taskTitle: string) {
  await page.goto("/tasks")
  const taskRow = page.getByRole("row").filter({ hasText: taskTitle })
  await expect(taskRow).toBeVisible()
  await taskRow.getByRole("button", { name: `${taskTitle}のアクションアイテムを表示`, exact: true }).click()
  const addButton = page.getByRole("button", { name: "今日に登録", exact: true })
  await expect(addButton).toBeEnabled()
  await addButton.click()
  await expect(page.getByText("今日のリストに追加しました。", { exact: true })).toBeVisible()
}

function civilDateInTimezone(timezone: string) {
  const parts = new Intl.DateTimeFormat("en-US", { timeZone: timezone, year: "numeric", month: "2-digit", day: "2-digit" }).formatToParts(new Date())
  const get = (type: Intl.DateTimeFormatPartTypes) => parts.find((part) => part.type === type)?.value || ""
  return `${get("year")}-${get("month")}-${get("day")}`
}

test("Today list supports adding, actual-time completion, reopening, and removal with API revisions", async ({ page, request }) => {
  test.setTimeout(60_000)
  const { headers } = await signIn(page, request)
  const { task, actionItem } = await createTaskActionItem(request, headers, "Today action task")
  await addTaskChildToToday(page, "Today action task")
  const { actionItem: reorderActionItem } = await createTaskActionItem(request, headers, "Today reorder task")
  await addTaskChildToToday(page, "Today reorder task")

  await page.goto("/today")
  const beforeReorder = await request.get("/api/today-todo", { headers })
  expect(beforeReorder.status()).toBe(200)
  const beforeRows = (await beforeReorder.json()).items.filter((item: { kind?: string; action_item_id?: string }) => item.kind === "action_item" && item.action_item_id) as Array<{ action_item_id: string; scheduled_start: string }>
  expect(beforeRows.map((item: { action_item_id: string }) => item.action_item_id)).toContain(actionItem.id)
  expect(beforeRows.map((item: { action_item_id: string }) => item.action_item_id)).toContain(reorderActionItem.id)
  const toMove = beforeRows[1].action_item_id === actionItem.id
    ? { id: actionItem.id, title: "Today action task action" }
    : { id: reorderActionItem.id, title: "Today reorder task action" }
  const reorderRow = page.getByRole("listitem").filter({ hasText: toMove.title })
  const reorderRequest = page.waitForResponse((response) => new URL(response.url()).pathname === "/api/today-todo:reorder" && response.status() === 200)
  await reorderRow.getByRole("button", { name: `${toMove.title}を前へ移動`, exact: true }).click()
  await reorderRequest
  const afterReorder = await request.get("/api/today-todo", { headers })
  expect(afterReorder.status()).toBe(200)
  const afterRows = (await afterReorder.json()).items.filter((item: { kind?: string; action_item_id?: string }) => item.kind === "action_item" && item.action_item_id) as Array<{ action_item_id: string; scheduled_start: string }>
  expect(afterRows.map((item: { action_item_id: string }) => item.action_item_id)).toEqual([...beforeRows].reverse().map((item: { action_item_id: string }) => item.action_item_id))
  const beforeBySource = new Map(beforeRows.map((item: { action_item_id: string; scheduled_start: string }) => [item.action_item_id, item.scheduled_start]))
  const afterBySource = new Map(afterRows.map((item: { action_item_id: string; scheduled_start: string }) => [item.action_item_id, item.scheduled_start]))
  const movedEarlier = afterRows[0]
  const movedLater = afterRows[1]
  expect(new Date(afterBySource.get(movedEarlier.action_item_id)!).getTime()).toBeLessThan(new Date(beforeBySource.get(movedEarlier.action_item_id)!).getTime())
  expect(new Date(afterBySource.get(movedLater.action_item_id)!).getTime()).toBeGreaterThan(new Date(beforeBySource.get(movedLater.action_item_id)!).getTime())

  const itemTitle = "Today action task action"
  const itemRow = page.getByRole("listitem").filter({ hasText: itemTitle })
  await expect(itemRow).toBeVisible()
  if (process.env.E2E_TODAY_SCREENSHOT) await page.screenshot({ path: process.env.E2E_TODAY_SCREENSHOT, fullPage: true })
  if (process.env.E2E_TODAY_HELP_SCREENSHOT) {
    await page.getByRole("button", { name: "今日のTodoリストが作られる仕組み", exact: true }).click()
    const help = page.getByRole("dialog")
    await expect(help).toBeVisible()
    const dimensions = await help.evaluate((element) => ({
      clientHeight: element.clientHeight,
      scrollHeight: element.scrollHeight,
      clientWidth: element.clientWidth,
      scrollWidth: element.scrollWidth,
    }))
    expect(dimensions.scrollHeight).toBeGreaterThan(dimensions.clientHeight)
    expect(dimensions.scrollWidth).toBeLessThanOrEqual(dimensions.clientWidth)
    await help.evaluate((element) => { element.scrollTop = element.scrollHeight })
    await expect(help.getByRole("button", { name: "閉じる", exact: true })).toBeInViewport()
    await help.evaluate((element) => { element.scrollTop = 0 })
    await page.screenshot({ path: process.env.E2E_TODAY_HELP_SCREENSHOT, fullPage: true })
    await help.getByRole("button", { name: "閉じる", exact: true }).click()
  }
  await itemRow.getByRole("button", { name: `${itemTitle}を完了`, exact: true }).click()
  await page.getByLabel("実績時間（分）", { exact: true }).fill("0")
  await page.getByRole("button", { name: "実績を保存して完了", exact: true }).click()
  await expect(itemRow.getByRole("button", { name: `${itemTitle}を未完了に戻す`, exact: true })).toBeVisible()
  const completedSource = await request.get(`/api/tasks/${task.id}/action-items`, { headers })
  expect(completedSource.status()).toBe(200)
  expect((await completedSource.json()).items[0]).toMatchObject({ id: actionItem.id, completed: true, actual_minutes: 0 })

  await itemRow.getByRole("button", { name: `${itemTitle}を未完了に戻す`, exact: true }).click()
  await expect(itemRow.getByRole("button", { name: `${itemTitle}を完了`, exact: true })).toBeVisible()
  await itemRow.getByRole("button", { name: "今日から外す", exact: true }).click()
  await expect(itemRow).toHaveCount(0)
  const todayResponse = await request.get("/api/today-todo", { headers })
  expect(todayResponse.status()).toBe(200)
  expect((await todayResponse.json()).items).not.toEqual(expect.arrayContaining([expect.objectContaining({ action_item_id: actionItem.id })]))
})

test("childless Task can be manually added, made urgent, completed with actual time, and reopened", async ({ page, request }) => {
  test.setTimeout(60_000)
  const { headers } = await signIn(page, request)
  const initialToday = await request.get("/api/today-todo", { headers })
  expect(initialToday.status()).toBe(200)

  const projectResponse = await request.post("/api/projects", {
    headers,
    data: { title: "Unallocated direct Task project", goal: "", description: "", type: "other", priority: "medium", start_date: null, end_date: null },
  })
  expect(projectResponse.status()).toBe(201)
  const project = await projectResponse.json() as { id: string }
  const taskResponse = await request.post(`/api/projects/${project.id}/tasks`, {
    headers,
    data: { title: "Direct Task work", description: "", priority: "medium", due_date: null, manual_estimated_minutes: 45 },
  })
  expect(taskResponse.status()).toBe(201)
  const task = await taskResponse.json() as { id: string; actual_minutes: number | null; action_item_count: number; manual_estimated_minutes: number }
  expect(task).toMatchObject({ actual_minutes: null, action_item_count: 0, manual_estimated_minutes: 45 })
  const beforeAdd = await request.get("/api/today-todo", { headers })
  const beforeAddItems = (await beforeAdd.json()).items as Array<{ kind: string; task_id?: string }>
  expect(beforeAddItems.some((item) => item.kind === "task" && item.task_id === task.id)).toBe(false)

  await page.goto("/tasks")
  const taskRow = page.getByRole("row").filter({ hasText: "Direct Task work" })
  await expect(taskRow.getByRole("button", { name: "今日に登録", exact: true })).toBeEnabled()
  await taskRow.getByRole("button", { name: "今日に登録", exact: true }).click()
  await expect(page.getByText("今日のリストに追加しました。", { exact: true })).toBeVisible()
  await page.getByRole("button", { name: "緊急にする", exact: true }).click()
  await expect(page.getByText("優先度を緊急にしました。", { exact: true })).toBeVisible()
  const urgentTask = await request.get(`/api/tasks/${task.id}`, { headers })
  expect(urgentTask.status()).toBe(200)
  expect((await urgentTask.json()).task).toMatchObject({ priority: "urgent", estimate_editable: true, can_add_action_item: true })

  await page.goto("/today")
  const row = page.getByRole("listitem").filter({ hasText: "Direct Task work" })
  await expect(row).toBeVisible()
  if (process.env.E2E_TASK_TODAY_SCREENSHOT) await page.screenshot({ path: process.env.E2E_TASK_TODAY_SCREENSHOT, fullPage: true })
  if (process.env.E2E_TASK_HELP_SCREENSHOT) {
    await page.getByRole("button", { name: "今日のTodoリストが作られる仕組み", exact: true }).click()
    const help = page.getByRole("dialog")
    await expect(help).toBeVisible()
    await help.evaluate((element) => { element.scrollTop = element.scrollHeight })
    await expect(help.getByRole("button", { name: "閉じる", exact: true })).toBeInViewport()
    await page.screenshot({ path: process.env.E2E_TASK_HELP_SCREENSHOT, fullPage: true })
    await help.getByRole("button", { name: "閉じる", exact: true }).click()
  }
  const completeResponse = page.waitForResponse((response) => new URL(response.url()).pathname.endsWith(":complete") && response.request().method() === "POST")
  await row.getByRole("button", { name: "Direct Task workを完了", exact: true }).click()
  await page.getByLabel("実績時間（分）", { exact: true }).fill("0")
  await page.getByRole("button", { name: "実績を保存して完了", exact: true }).click()
  expect((await completeResponse).status()).toBe(200)
  await expect(row.getByRole("button", { name: "Direct Task workを未完了に戻す", exact: true })).toBeVisible()
  const completed = await request.get(`/api/tasks/${task.id}`, { headers })
  expect(completed.status()).toBe(200)
  expect((await completed.json()).task).toMatchObject({ status: "done", actual_minutes: 0, started: true, estimate_editable: false, can_add_action_item: false })

  const reopenResponse = page.waitForResponse((response) => new URL(response.url()).pathname.endsWith(":reopen") && response.request().method() === "POST")
  await row.getByRole("button", { name: "Direct Task workを未完了に戻す", exact: true }).click()
  expect((await reopenResponse).status()).toBe(200)
  await expect(row.getByRole("button", { name: "Direct Task workを完了", exact: true })).toBeVisible()
  const reopened = await request.get(`/api/tasks/${task.id}`, { headers })
  expect(reopened.status()).toBe(200)
  expect((await reopened.json()).task).toMatchObject({ actual_minutes: null, started: false, estimate_editable: true, can_add_action_item: true })
})

test("automatically plans childless project Tasks in allocations and projectless Tasks in their complement", async ({ page, request }) => {
  const { headers } = await signIn(page, request)
  const userResponse = await request.get("/api/users/me", { headers })
  expect(userResponse.status()).toBe(200)
  const user = await userResponse.json() as { timezone: string }
  const today = civilDateInTimezone(user.timezone)
  const startTime = "10:00"
  const endTime = "11:00"

  const projectResponse = await request.post("/api/projects", {
    headers,
    data: { title: "Allocated direct Task project", goal: "", description: "", type: "other", priority: "medium", start_date: null, end_date: null },
  })
  expect(projectResponse.status()).toBe(201)
  const project = await projectResponse.json() as { id: string }
  const projectTaskResponse = await request.post(`/api/projects/${project.id}/tasks`, {
    headers,
    data: { title: "Allocated direct work", description: "", priority: "high", due_date: null, manual_estimated_minutes: 20 },
  })
  expect(projectTaskResponse.status()).toBe(201)
  const projectTask = await projectTaskResponse.json() as { id: string }
  const projectlessTaskResponse = await request.post("/api/tasks", {
    headers,
    data: { title: "Complement direct work", description: "", priority: "medium", due_date: null, manual_estimated_minutes: 20 },
  })
  expect(projectlessTaskResponse.status()).toBe(201)
  const projectlessTask = await projectlessTaskResponse.json() as { id: string }
  const projectTaskStateResponse = await request.get(`/api/tasks/${projectTask.id}`, { headers })
  expect(projectTaskStateResponse.status()).toBe(200)
  const projectTaskState = await projectTaskStateResponse.json() as { task: { project_id: string; status: string; manual_estimated_minutes: number; action_item_count: number } }
  expect(projectTaskState.task).toMatchObject({ project_id: project.id, status: "open", manual_estimated_minutes: 20, action_item_count: 0 })

  const createAllocationResponse = await request.post("/api/allocations", {
    headers,
    data: {
      project_id: project.id,
      timezone: user.timezone,
      start_at: new Date(timestampAtLocalTime(today, startTime, user.timezone)).toISOString(),
      end_at: new Date(timestampAtLocalTime(today, endTime, user.timezone)).toISOString(),
      repeat_weekly: false,
    },
  })
  expect(createAllocationResponse.status()).toBe(201)
  const allocationWeekResponse = await request.get(`/api/allocation-weeks/${mondayOf(today)}`, { headers })
  expect(allocationWeekResponse.status()).toBe(200)
  const allocationWeek = await allocationWeekResponse.json() as { allocations: Array<{ project_id: string; occurrence_date: string }> }
  expect(allocationWeek.allocations.some((item) => item.project_id === project.id && item.occurrence_date === today)).toBe(true)

  const beforeRegenerate = await request.get("/api/today-todo", { headers })
  expect(beforeRegenerate.status()).toBe(200)
  const currentTodo = await beforeRegenerate.json() as { revision: number }
  const regenerateResponse = await request.post("/api/today-todo:regenerate", {
    headers: { ...headers, "If-Match": `"${currentTodo.revision}"` },
  })
  expect(regenerateResponse.status()).toBe(200)
  const snapshot = await regenerateResponse.json() as { items: Array<{ kind: string; task_id?: string; scheduled_start?: string; scheduled_end?: string }> }
  const projectRow = snapshot.items.find((item) => item.kind === "task" && item.task_id === projectTask.id)
  const projectlessRow = snapshot.items.find((item) => item.kind === "task" && item.task_id === projectlessTask.id)
  expect(projectRow).toBeTruthy()
  expect(projectlessRow).toBeTruthy()
  const allocationStart = timestampAtLocalTime(today, startTime, user.timezone)
  const allocationEnd = timestampAtLocalTime(today, endTime, user.timezone)
  const projectStart = new Date(projectRow!.scheduled_start!).getTime()
  const projectEnd = new Date(projectRow!.scheduled_end!).getTime()
  expect(projectStart).toBeGreaterThanOrEqual(allocationStart)
  expect(projectEnd).toBeLessThanOrEqual(allocationEnd)
  const projectlessStart = new Date(projectlessRow!.scheduled_start!).getTime()
  const projectlessEnd = new Date(projectlessRow!.scheduled_end!).getTime()
  expect(projectlessEnd <= allocationStart || projectlessStart >= allocationEnd).toBe(true)
})

test("regeneration cancel keeps manual Today work and confirmation resets it", async ({ page, request }) => {
  test.setTimeout(60_000)
  const { headers } = await signIn(page, request)
  const projectResponse = await request.post("/api/projects", {
    headers,
    data: { title: "No allocation project", goal: "", description: "", type: "other", priority: "low", start_date: null, end_date: null },
  })
  expect(projectResponse.status()).toBe(201)
  const project = await projectResponse.json() as { id: string }
  const { actionItem } = await createTaskActionItem(request, headers, "Manual plan item", 30, project.id)
  await addTaskChildToToday(page, "Manual plan item")

  await page.goto("/today")
  const row = page.getByRole("listitem").filter({ hasText: "Manual plan item action" })
  await expect(row).toBeVisible()
  const beforeResponse = await request.get("/api/today-todo", { headers })
  expect(beforeResponse.status()).toBe(200)
  const before = await beforeResponse.json() as { revision: number; items: Array<{ action_item_id?: string }> }
  expect(before.items.some((item) => item.action_item_id === actionItem.id)).toBe(true)

  await page.getByRole("button", { name: "今日のリストを再生成", exact: true }).click()
  const confirm = page.getByRole("alertdialog")
  await expect(confirm).toBeVisible()
  await confirm.getByRole("button", { name: "キャンセル", exact: true }).click()
  await expect(row).toBeVisible()
  const afterCancel = await request.get("/api/today-todo", { headers })
  const unchanged = await afterCancel.json() as { revision: number }
  expect(unchanged.revision).toBe(before.revision)

  await page.getByRole("button", { name: "今日のリストを再生成", exact: true }).click()
  await page.getByRole("alertdialog").getByRole("button", { name: "再生成する", exact: true }).click()
  await expect(page.getByText("今日のリストを再生成しました。", { exact: true })).toBeVisible()
  await expect(row).toHaveCount(0)
  const regenerated = await request.get("/api/today-todo", { headers })
  expect(regenerated.status()).toBe(200)
  expect((await regenerated.json()).items).not.toEqual(expect.arrayContaining([expect.objectContaining({ action_item_id: actionItem.id })]))
})

function timestampAtLocalTime(date: string, time: string, timezone: string) {
  const noon = new Date(`${date}T12:00:00Z`)
  const parts = new Intl.DateTimeFormat("en-US", { timeZone: timezone, timeZoneName: "longOffset" }).formatToParts(noon)
  const offset = parts.find((part) => part.type === "timeZoneName")?.value?.match(/GMT([+-]\d{2}:\d{2})/)?.[1] ?? "+00:00"
  return new Date(`${date}T${time}:00${offset}`).getTime()
}

function mondayOf(value: string) {
  const [year, month, day] = value.split("-").map(Number)
  const date = new Date(Date.UTC(year!, month! - 1, day!))
  date.setUTCDate(date.getUTCDate() - ((date.getUTCDay() + 6) % 7))
  return `${date.getUTCFullYear()}-${String(date.getUTCMonth() + 1).padStart(2, "0")}-${String(date.getUTCDate()).padStart(2, "0")}`
}

test("Schedule source change can be canceled without saving or confirmed into Today", async ({ page, request }) => {
  test.setTimeout(60_000)
  const { headers } = await signIn(page, request)
  await createTaskActionItem(request, headers, "Schedule impact task")
  await addTaskChildToToday(page, "Schedule impact task")
  const userResponse = await request.get("/api/users/me", { headers })
  expect(userResponse.status()).toBe(200)
  const user = await userResponse.json() as { timezone: string }
  const date = civilDateInTimezone(user.timezone || "UTC")
  const from = `${date}T00:00:00Z`
  const nextDay = new Date(`${date}T00:00:00Z`)
  nextDay.setUTCDate(nextDay.getUTCDate() + 2)
  const to = nextDay.toISOString()
  const schedulesBefore = await request.get(`/api/schedules?from=${encodeURIComponent(from)}&to=${encodeURIComponent(to)}`, { headers })
  expect(schedulesBefore.status()).toBe(200)

  await page.goto("/schedules/new")
  await page.getByLabel("タイトル", { exact: true }).fill("Today confirmed schedule")
  await page.getByLabel("開始日", { exact: true }).fill(date)
  await page.getByLabel("終了日", { exact: true }).fill(date)
  await page.getByLabel("開始時刻", { exact: true }).fill("23:00")
  await page.getByLabel("終了時刻", { exact: true }).fill("23:30")
  await page.getByRole("button", { name: "作成する", exact: true }).click()

  const confirm = page.getByRole("alertdialog")
  await expect(confirm).toBeVisible()
  await expect(page.getByLabel("タイトル", { exact: true })).toHaveValue("Today confirmed schedule")
  await confirm.getByRole("button", { name: "キャンセル", exact: true }).click()
  await expect(page).toHaveURL("/schedules/new")
  await expect(page.getByLabel("タイトル", { exact: true })).toHaveValue("Today confirmed schedule")
  const schedulesAfterCancel = await request.get(`/api/schedules?from=${encodeURIComponent(from)}&to=${encodeURIComponent(to)}`, { headers })
  expect((await schedulesAfterCancel.json()).items).not.toEqual(expect.arrayContaining([expect.objectContaining({ title: "Today confirmed schedule" })]))

  await page.getByRole("button", { name: "作成する", exact: true }).click()
  await page.getByRole("alertdialog").getByRole("button", { name: "再生成する", exact: true }).click()
  await expect(page).toHaveURL(/\/schedules\?date=/)
  await page.goto("/today")
  await expect(page.getByRole("listitem").filter({ hasText: "Today confirmed schedule" })).toBeVisible()
  const schedulesAfterSave = await request.get(`/api/schedules?from=${encodeURIComponent(from)}&to=${encodeURIComponent(to)}`, { headers })
  expect((await schedulesAfterSave.json()).items).toEqual(expect.arrayContaining([expect.objectContaining({ title: "Today confirmed schedule" })]))
})

test("Inbox child ActionItem can be added to Today inline", async ({ page, request }) => {
  test.setTimeout(60_000)
  const { headers } = await signIn(page, request)
  await createTaskActionItem(request, headers, "Inbox child task")
  await page.goto("/inbox")
  const taskRow = page.getByRole("listitem").filter({ hasText: "Inbox child task" })
  await expect(taskRow).toBeVisible()
  await taskRow.getByRole("button", { name: "Inbox child taskのActionItemを表示", exact: true }).click()
  const childRow = page.locator("li").filter({ hasText: "Inbox child task action" })
  await expect(childRow).toBeVisible()
  await childRow.getByRole("button", { name: "今日に登録", exact: true }).click()
  await expect(page.getByText("今日のリストに追加しました。", { exact: true })).toBeVisible()
  await page.goto("/today")
  await expect(page.getByRole("listitem").filter({ hasText: "Inbox child task action" })).toBeVisible()
})
