import { expect, test } from "@playwright/test"
import { createAccount, fillCredentials } from "./helpers/auth.ts"

async function signIn(page: import("@playwright/test").Page, request: import("@playwright/test").APIRequestContext) {
  const credentials = await createAccount(request)
  await page.goto("/login")
  await fillCredentials(page, credentials)
  await page.getByRole("button", { name: "ログイン", exact: true }).click()
  await expect(page).toHaveURL("/today")
}

test("creates a Task with an ActionItem and reflects completion in list progress", async ({ page, request }) => {
  await signIn(page, request)
  await page.goto("/tasks")
  await expect(page.getByRole("heading", { name: "タスク", exact: true })).toBeVisible()

  await page.getByRole("button", { name: "タスクを追加", exact: true }).click()
  await expect(page).toHaveURL("/tasks/new")
  const createForm = page.getByRole("form", { name: "タスクを追加" })
  await createForm.getByLabel("タスク名 必須").fill("E2E project plan")
  await page.getByLabel("優先度", { exact: true }).selectOption("high")
  await page.getByRole("button", { name: "アクションアイテムを追加", exact: true }).click()
  await page.getByLabel("アクションアイテム名 1", { exact: true }).fill("Write first draft")
  await page.getByLabel("見積り 1", { exact: true }).fill("1:00")
  await page.getByLabel("優先度 1", { exact: true }).selectOption("medium")
  await page.getByRole("button", { name: "追加する", exact: true }).click()

  await expect(page.getByText("タスクを追加しました。", { exact: true })).toBeVisible()
  const taskRow = page.getByRole("row").filter({ hasText: "E2E project plan" })
  await expect(taskRow).toBeVisible()
  await expect(taskRow).toContainText("1:00")

  const token = await page.evaluate(() => localStorage.getItem("personal_access_token"))
  expect(token).toBeTruthy()
  const headers = { Authorization: `Bearer ${token}` }
  const taskList = await page.request.get("/api/tasks?status=all&title=E2E%20project%20plan", { headers })
  expect(taskList.status()).toBe(200)
  const createdTask = (await taskList.json()).items[0]
  expect(createdTask).toMatchObject({ title: "E2E project plan", priority: "high", estimated_minutes: 60, estimate_source: "action_items" })
  const actionItems = await page.request.get(`/api/tasks/${createdTask.id}/action-items`, { headers })
  expect(actionItems.status()).toBe(200)
  expect((await actionItems.json()).items[0]).toMatchObject({ title: "Write first draft", priority: "medium", estimated_minutes: 60 })

  await taskRow.getByRole("button", { name: "タスクを編集: E2E project plan", exact: true }).click()
  await expect(page).toHaveURL(new RegExp(`/tasks/${createdTask.id}/edit$`))
  await expect(page.getByRole("heading", { name: "タスクを編集", exact: true })).toBeVisible()
  await page.getByRole("form", { name: "タスクを編集" }).getByLabel("タスク名 必須").fill("E2E revised plan")
  await page.getByLabel(/^アクションアイテム名 1/).fill("Write revised draft")
  await page.getByLabel(/^見積り 1/).fill("1:15")
  await page.getByLabel(/^優先度 1/).selectOption("low")
  await page.getByRole("button", { name: "アクションアイテムを追加", exact: true }).click()
  await page.getByLabel(/^アクションアイテム名 2/).fill("Review draft")
  await page.getByLabel(/^見積り 2/).fill("0:30")
  await page.getByRole("button", { name: "変更を保存", exact: true }).click()
  await expect(page).toHaveURL("/tasks")
  await expect(page.getByText("タスクを更新しました。", { exact: true })).toBeVisible()
  const revisedTaskRow = page.getByRole("row").filter({ hasText: "E2E revised plan" })
  await expect(revisedTaskRow).toContainText("1:45")

  const revisedTaskList = await page.request.get(`/api/tasks?status=all&title=E2E%20revised%20plan`, { headers })
  expect(revisedTaskList.status()).toBe(200)
  const revisedTask = (await revisedTaskList.json()).items[0]
  expect(revisedTask).toMatchObject({ title: "E2E revised plan", estimated_minutes: 105, estimate_source: "action_items" })
  const revisedActionItems = await page.request.get(`/api/tasks/${createdTask.id}/action-items`, { headers })
  expect(revisedActionItems.status()).toBe(200)
  const revisedActionItemRows = (await revisedActionItems.json()).items
  expect(revisedActionItemRows.map((item: { title: string; estimated_minutes: number }) => [item.title, item.estimated_minutes])).toEqual([
    ["Write revised draft", 75],
    ["Review draft", 30],
  ])
  expect(revisedActionItemRows[0]).toMatchObject({ priority: "low" })

  await revisedTaskRow.getByRole("button", { name: "タスクを編集: E2E revised plan", exact: true }).click()
  await expect(page).toHaveURL(new RegExp(`/tasks/${createdTask.id}/edit$`))
  const editForm = page.getByRole("form", { name: "タスクを編集" })
  await editForm.getByLabel("タスク名 必須").fill("Draft survives conflict")
  const currentChild = revisedActionItemRows[0]
  const independentChildEdit = await page.request.patch(`/api/tasks/${createdTask.id}/action-items/${currentChild.series_id}`, {
    headers,
    data: { scope: "current", occurrence_date: currentChild.occurrence_date, title: "External child edit" },
  })
  expect(independentChildEdit.status()).toBe(200)
  await page.getByRole("button", { name: "変更を保存", exact: true }).click()
  await expect(page.getByRole("alert")).toContainText("別の変更が先に保存されています")
  await expect(editForm.getByLabel("タスク名 必須")).toHaveValue("Draft survives conflict")
  await page.getByRole("button", { name: "最新の内容を読み込む", exact: true }).click()
  await expect(page.getByRole("form", { name: "タスクを編集" }).getByLabel("タスク名 必須")).toHaveValue("E2E revised plan")
  await page.getByRole("button", { name: "アクションアイテム 2 を削除", exact: true }).click()
  await page.getByRole("button", { name: "変更を保存", exact: true }).click()
  await expect(page).toHaveURL(/\/tasks(?:\?status=open)?$/)
  const afterDelete = await page.request.get(`/api/tasks/${createdTask.id}/action-items`, { headers })
  expect(afterDelete.status()).toBe(200)
  expect((await afterDelete.json()).items.map((item: { title: string }) => item.title)).toEqual(["External child edit"])
  const afterDeleteTaskList = await page.request.get(`/api/tasks?status=all&title=E2E%20revised%20plan`, { headers })
  expect(afterDeleteTaskList.status()).toBe(200)
  expect((await afterDeleteTaskList.json()).items[0]).toMatchObject({ estimated_minutes: 75, action_item_count: 1 })

  await page.getByRole("button", { name: "E2E revised planのアクションアイテムを表示", exact: true }).click()
  await expect(page.getByText("External child edit", { exact: true })).toBeVisible()
  await expect(page.getByRole("button", { name: "今日に登録", exact: true })).toBeDisabled()
  const progress = page.getByRole("progressbar", { name: "アクションアイテムの進捗" }).first()
  await expect(progress).toHaveAttribute("aria-valuetext", "0 / 1")

  const completeButton = page.getByRole("button", { name: "アクションアイテムを完了にする", exact: true })
  await completeButton.click()
  await expect(progress).toHaveAttribute("aria-valuetext", "1 / 1")
  await expect(progress).toHaveAttribute("aria-valuenow", "100")
  await expect(page.getByText("アクションアイテムの状態を更新しました。", { exact: true })).toBeVisible()

  await page.getByRole("button", { name: "アクションアイテムを未完了に戻す", exact: true }).click()
  await expect(progress).toHaveAttribute("aria-valuetext", "0 / 1")
  await expect(progress).toHaveAttribute("aria-valuenow", "0")

  await page.getByPlaceholder("タスク名で絞り込み").fill("E2E revised plan")
  await expect(revisedTaskRow).toBeVisible()
  await page.getByPlaceholder("タスク名で絞り込み").fill("missing task")
  await expect(page.getByText("条件に一致するタスクはありません。", { exact: true })).toBeVisible()
  await page.getByPlaceholder("タスク名で絞り込み").fill("")
  await page.getByRole("button", { name: /^進行中/ }).click()
  await expect(page.getByText("条件に一致するタスクはありません。", { exact: true })).toBeVisible()
  await page.getByRole("button", { name: /^未着手/ }).click()
  await expect(revisedTaskRow).toBeVisible()
})

test("shows only failed ActionItem save and retries without duplicating successful saves", async ({ page, request }) => {
  await signIn(page, request)
  await page.goto("/tasks/new")
  const createForm = page.getByRole("form", { name: "タスクを追加" })
  await createForm.getByLabel("タスク名 必須").fill("Partial save task")
  await page.getByRole("button", { name: "アクションアイテムを追加", exact: true }).click()
  await page.getByLabel("アクションアイテム名 1", { exact: true }).fill("Update succeeds")
  await page.getByLabel("見積り 1", { exact: true }).fill("0:30")
  await page.getByRole("button", { name: "追加する", exact: true }).click()
  await expect(page.getByText("タスクを追加しました。", { exact: true })).toBeVisible()

  const token = await page.evaluate(() => localStorage.getItem("personal_access_token"))
  expect(token).toBeTruthy()
  const headers = { Authorization: `Bearer ${token}` }
  const tasksResponse = await page.request.get("/api/tasks?status=all&title=Partial%20save%20task", { headers })
  expect(tasksResponse.status()).toBe(200)
  const taskRows = (await tasksResponse.json()).items
  expect(taskRows).toHaveLength(1)
  const task = taskRows[0]
  const profileResponse = await page.request.get("/api/users/me", { headers })
  expect(profileResponse.status()).toBe(200)
  const profile = await profileResponse.json()
  const childrenResponse = await page.request.get(`/api/tasks/${task.id}/action-items`, { headers })
  expect(childrenResponse.status()).toBe(200)
  const children = (await childrenResponse.json()).items
  expect(children).toHaveLength(1)
  const first = children[0]
  const firstSeriesId = first.series_id || first.id
  const today = new Intl.DateTimeFormat("en-CA", { timeZone: profile.timezone }).format(new Date())
  const weekday = ["sun", "mon", "tue", "wed", "thu", "fri", "sat"][new Date(`${today}T00:00:00Z`).getUTCDay()]
  const recurringResponse = await page.request.post(`/api/tasks/${task.id}/action-items`, {
    headers,
    data: { title: "Delete fails once", estimated_minutes: 45, due_date: today, interval_weeks: 1, frequencies: [weekday] },
  })
  expect(recurringResponse.status()).toBe(201)
  const recurringRoot = await recurringResponse.json()
  const seriesId = recurringRoot.series_id || recurringRoot.id
  const recurringListResponse = await page.request.get(`/api/tasks/${task.id}/action-items`, { headers })
  expect(recurringListResponse.status()).toBe(200)
  const recurringRows = (await recurringListResponse.json()).items
  const second = recurringRows.find((item: { series_id: string; occurrence_date: string; title: string }) => item.series_id === seriesId && item.occurrence_date === today && item.title === "Delete fails once")
  expect(second).toBeTruthy()
  const secondIndex = recurringRows.indexOf(second) + 1
  expect(secondIndex).toBeGreaterThan(0)
  const firstIndex = recurringRows.findIndex((item: { title: string; series_id: string; id: string }) => item.title === "Update succeeds" && (item.series_id || item.id) === firstSeriesId) + 1
  expect(firstIndex).toBeGreaterThan(0)
  const firstInput = page.getByLabel(new RegExp(`^アクションアイテム名 ${firstIndex}(?: \\(|$)`))

  const requests = { taskCreate: 0, firstItemUpdate: 0, failedSkip: 0 }
  const skipStatuses: number[] = []
  page.on("request", (outgoing) => {
    if (outgoing.method() === "POST" && outgoing.url().endsWith("/api/tasks")) requests.taskCreate += 1
    if (outgoing.method() === "PATCH" && outgoing.url().endsWith(`/api/tasks/${task.id}/action-items/${firstSeriesId}`)) requests.firstItemUpdate += 1
    if (outgoing.method() === "POST" && outgoing.url().endsWith(`/api/tasks/${task.id}/action-items/${seriesId}:skip`)) requests.failedSkip += 1
  })
  page.on("response", (incoming) => {
    if (incoming.url().endsWith(`/api/tasks/${task.id}/action-items/${seriesId}:skip`)) skipStatuses.push(incoming.status())
  })

  const taskRow = page.getByRole("row").filter({ hasText: "Partial save task" })
  await taskRow.getByRole("button", { name: "タスクを編集: Partial save task", exact: true }).click()
  await firstInput.fill("Update saved once")
  await page.getByRole("button", { name: `アクションアイテム ${secondIndex} を削除`, exact: true }).click()

  const completeSecond = await page.request.post(`/api/tasks/${task.id}/action-items/${second.series_id}:complete`, {
    headers,
    data: { occurrence_date: second.occurrence_date },
  })
  expect(completeSecond.status()).toBe(200)

  await page.getByRole("button", { name: "変更を保存", exact: true }).click()
  const failedActionItem = page.getByRole("group").filter({ hasText: "Delete fails once" })
  await expect(failedActionItem.getByText("Delete fails once: アクションアイテムを保存できませんでした。", { exact: true })).toBeVisible()
  await expect(firstInput).toHaveValue("Update saved once")

  const savedAfterFailure = await page.request.get(`/api/tasks/${task.id}/action-items`, { headers })
  expect(savedAfterFailure.status()).toBe(200)
  const savedRows = (await savedAfterFailure.json()).items
  expect(savedRows.filter((item: { title: string }) => item.title === "Update saved once")).toHaveLength(1)
  expect(savedRows.some((item: { title: string; occurrence_date: string }) => item.title === "Delete fails once" && item.occurrence_date === today)).toBe(true)
  expect(requests.taskCreate).toBe(0)
  expect(requests.firstItemUpdate).toBe(1)
  expect(requests.failedSkip).toBe(1)
  expect(skipStatuses).toEqual([409])

  const reopenSecond = await page.request.post(`/api/tasks/${task.id}/action-items/${seriesId}:reopen`, {
    headers,
    data: { occurrence_date: second.occurrence_date },
  })
  expect(reopenSecond.status()).toBe(200)
  await page.getByRole("button", { name: "変更を保存", exact: true }).click()
  await expect(page).toHaveURL(/\/tasks(?:\?status=open)?$/)

  const finalTasks = await page.request.get("/api/tasks?status=all&title=Partial%20save%20task", { headers })
  expect(finalTasks.status()).toBe(200)
  expect((await finalTasks.json()).items).toHaveLength(1)
  const finalChildren = await page.request.get(`/api/tasks/${task.id}/action-items`, { headers })
  expect(finalChildren.status()).toBe(200)
  const finalRows = (await finalChildren.json()).items
  expect(finalRows.filter((item: { title: string }) => item.title === "Update saved once")).toHaveLength(1)
  expect(finalRows.some((item: { title: string; occurrence_date: string }) => item.title === "Delete fails once" && item.occurrence_date === today)).toBe(false)
  expect(requests.taskCreate).toBe(0)
  expect(requests.firstItemUpdate).toBe(1)
  expect(requests.failedSkip).toBe(2)
  expect(skipStatuses).toEqual([409, 200])
})
