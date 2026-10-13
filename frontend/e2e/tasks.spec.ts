import { expect, test } from "@playwright/test"
import { createAccount, fillCredentials } from "./helpers/auth.ts"

async function signIn(page: import("@playwright/test").Page, request: import("@playwright/test").APIRequestContext) {
  const credentials = await createAccount(request)
  await page.goto("/login")
  await fillCredentials(page, credentials)
  const todayResponse = page.waitForResponse((response) => new URL(response.url()).pathname === "/api/today-todo" && response.request().method() === "GET")
  await page.getByRole("button", { name: "ログイン", exact: true }).click()
  await expect(page).toHaveURL("/today")
  expect((await todayResponse).status()).toBe(200)
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
  expect((await actionItems.json()).items[0]).toMatchObject({ title: "Write first draft", priority: "medium", estimated_minutes: 60, completed: false, started: false, estimate_editable: true })

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
  const independentChildEdit = await page.request.patch(`/api/tasks/${createdTask.id}/action-items/${currentChild.id}`, {
    headers,
    data: { title: "External child edit" },
  })
  expect(independentChildEdit.status()).toBe(200)
  await page.getByRole("button", { name: "変更を保存", exact: true }).click()
  await expect(page.getByRole("alert")).toContainText("別の変更が先に保存されています")
  await expect(page.getByText("一部の変更を保存できませんでした。エラー項目を修正または再試行してください。", { exact: true })).toHaveCount(0)
  await expect(editForm.getByLabel("タスク名 必須")).toHaveValue("Draft survives conflict")
  await page.getByRole("button", { name: "最新の内容を読み込む", exact: true }).click()
  const reloadDialog = page.getByRole("alertdialog", { name: "最新の内容を読み込みますか？" })
  await reloadDialog.getByRole("button", { name: "最新の内容を読み込む", exact: true }).click()
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
  await expect(page.getByRole("button", { name: "今日に登録", exact: true })).toBeEnabled()
  const progress = page.getByRole("progressbar", { name: "アクションアイテムの進捗" }).first()
  await expect(progress).toHaveAttribute("aria-valuetext", "0 / 1")

  const completeButton = page.getByRole("button", { name: "アクションアイテムを完了にする", exact: true })
  await completeButton.click()
  const actualDialog = page.getByRole("dialog")
  await expect(actualDialog).toBeVisible()
  await actualDialog.getByLabel("実績時間（分）", { exact: true }).fill("42")
  await actualDialog.getByRole("button", { name: "実績を保存して完了", exact: true }).click()
  await expect(progress).toHaveAttribute("aria-valuetext", "1 / 1")
  await expect(progress).toHaveAttribute("aria-valuenow", "100")
  await expect(page.getByText("アクションアイテムの状態を更新しました。", { exact: true })).toBeVisible()

  await page.getByRole("button", { name: "タスクを編集: E2E revised plan", exact: true }).click()
  const completedEditForm = page.getByRole("form", { name: "タスクを編集" })
  await expect(completedEditForm.getByLabel(/^見積り 1/)).toBeDisabled()
  await completedEditForm.getByLabel(/^実績（分） 1/).fill("48")
  await page.getByRole("button", { name: "変更を保存", exact: true }).click()
  await expect(page).toHaveURL("/tasks")
  const actualAfterEdit = await page.request.get(`/api/tasks/${createdTask.id}/action-items`, { headers })
  expect(actualAfterEdit.status()).toBe(200)
  expect((await actualAfterEdit.json()).items[0]).toMatchObject({ actual_minutes: 48, estimated_minutes: 75 })

  await page.getByRole("button", { name: "E2E revised planのアクションアイテムを表示", exact: true }).click()
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

test("saves first-child and last-child changes for a manually planned direct Task without Todo regeneration", async ({ page, request }) => {
  test.setTimeout(120_000)
  await signIn(page, request)
  const token = await page.evaluate(() => localStorage.getItem("personal_access_token"))
  expect(token).toBeTruthy()
  const headers = { Authorization: `Bearer ${token}` }
  const initialToday = await request.get("/api/today-todo", { headers })
  expect(initialToday.status()).toBe(200)
  const projectResponse = await request.post("/api/projects", {
    headers,
    data: { title: "Structural confirmation project", goal: "", description: "", type: "other", priority: "medium", start_date: null, end_date: null },
  })
  expect(projectResponse.status()).toBe(201)
  const project = await projectResponse.json() as { id: string }
  const taskResponse = await request.post(`/api/projects/${project.id}/tasks`, {
    headers,
    data: { title: "Structural direct Task", description: "", priority: "medium", due_date: null, manual_estimated_minutes: 30 },
  })
  expect(taskResponse.status()).toBe(201)
  const task = await taskResponse.json() as { id: string }

  await page.goto("/tasks")
  const taskRow = page.getByRole("row").filter({ hasText: "Structural direct Task" })
  await taskRow.getByRole("button", { name: "今日に登録", exact: true }).click()
  await expect(page.getByText("今日のリストに追加しました。", { exact: true })).toBeVisible()
  await taskRow.getByRole("button", { name: "タスクを編集: Structural direct Task", exact: true }).click()
  await page.getByRole("button", { name: "アクションアイテムを追加", exact: true }).click()
  await page.getByLabel("アクションアイテム名 1", { exact: true }).fill("Structural child")
  await page.getByLabel("見積り 1", { exact: true }).fill("0:45")
  const createConfirmedChild = page.waitForResponse((response) =>
    response.request().method() === "POST" && response.url().endsWith(`/api/tasks/${task.id}/action-items`) && response.status() === 201,
  )
  await page.getByRole("button", { name: "変更を保存", exact: true }).click()

  await expect(page.getByRole("alertdialog")).toHaveCount(0)
  await createConfirmedChild
  const childrenAfterCreate = await request.get(`/api/tasks/${task.id}/action-items`, { headers })
  expect(childrenAfterCreate.status()).toBe(200)
  const child = (await childrenAfterCreate.json()).items[0] as { id: string; title: string }
  expect(child.title).toBe("Structural child")
  const todoAfterChildCreate = await request.get("/api/today-todo", { headers })
  expect(todoAfterChildCreate.status()).toBe(200)
  const todoRowsAfterChildCreate = (await todoAfterChildCreate.json()).items
  expect(todoRowsAfterChildCreate).toEqual(expect.arrayContaining([expect.objectContaining({ kind: "task", task_id: task.id })]))
  expect(todoRowsAfterChildCreate).not.toEqual(expect.arrayContaining([expect.objectContaining({ action_item_id: child.id })]))

  await page.goto("/tasks")
  const childBearingRow = page.getByRole("row").filter({ hasText: "Structural direct Task" })
  await childBearingRow.getByRole("button", { name: "Structural direct Taskのアクションアイテムを表示", exact: true }).click()
  const childRow = page.getByRole("row").filter({ hasText: "Structural child" })
  await expect(childRow.getByRole("button", { name: "今日に登録", exact: true })).toBeEnabled()
  await childRow.getByRole("button", { name: "今日に登録", exact: true }).click()
  await expect(page.getByText("今日のリストに追加しました。", { exact: true })).toBeVisible()

  await childBearingRow.getByRole("button", { name: "タスクを編集: Structural direct Task", exact: true }).click()
  await page.getByLabel("アクションアイテム 1 を削除", { exact: true }).click()
  await page.getByLabel(/見積り/).fill("1:00")
  const deleteConfirmedChild = page.waitForResponse((response) =>
    response.request().method() === "DELETE" && response.url().endsWith(`/api/tasks/${task.id}/action-items/${child.id}`) && response.status() >= 200 && response.status() < 300,
  )
  await page.getByRole("button", { name: "変更を保存", exact: true }).click()
  await expect(page.getByRole("alertdialog")).toHaveCount(0)
  await deleteConfirmedChild
  const childrenAfterLastDelete = await request.get(`/api/tasks/${task.id}/action-items`, { headers })
  expect(childrenAfterLastDelete.status()).toBe(200)
  expect((await childrenAfterLastDelete.json()).items).toHaveLength(0)
  const taskAfterLastDelete = await request.get(`/api/tasks/${task.id}`, { headers })
  expect((await taskAfterLastDelete.json()).task).toMatchObject({ manual_estimated_minutes: 60, action_item_count: 0, estimate_source: "manual" })
})

test("retries failed ActionItem deletion without duplicating successful saves", async ({ page, request }) => {
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
  const task = (await tasksResponse.json()).items[0]
  const childrenResponse = await page.request.get(`/api/tasks/${task.id}/action-items`, { headers })
  expect(childrenResponse.status()).toBe(200)
  const first = (await childrenResponse.json()).items[0]
  const createSecond = await page.request.post(`/api/tasks/${task.id}/action-items`, {
    headers,
    data: { title: "Delete fails once", estimated_minutes: 45 },
  })
  expect(createSecond.status()).toBe(201)
  const second = await createSecond.json()

  const requests = { taskCreate: 0, firstItemUpdate: 0, deleteSecond: 0 }
  const deleteStatuses: number[] = []
  page.on("request", (outgoing) => {
    if (outgoing.method() === "POST" && outgoing.url().endsWith("/api/tasks")) requests.taskCreate += 1
    if (outgoing.method() === "PATCH" && outgoing.url().endsWith(`/api/tasks/${task.id}/action-items/${first.id}`)) requests.firstItemUpdate += 1
    if (outgoing.method() === "DELETE" && outgoing.url().endsWith(`/api/tasks/${task.id}/action-items/${second.id}`)) requests.deleteSecond += 1
  })
  page.on("response", (incoming) => {
    if (incoming.url().endsWith(`/api/tasks/${task.id}/action-items/${second.id}`) && incoming.request().method() === "DELETE") deleteStatuses.push(incoming.status())
  })

  const taskRow = page.getByRole("row").filter({ hasText: "Partial save task" })
  await taskRow.getByRole("button", { name: "タスクを編集: Partial save task", exact: true }).click()
  const firstInput = page.getByLabel("アクションアイテム名 1", { exact: true })
  await firstInput.fill("Update saved once")
  await page.getByLabel("アクションアイテム名 2", { exact: true }).fill("Delete fails once")
  await page.getByRole("button", { name: "アクションアイテム 2 を削除", exact: true }).click()

  const completeSecond = await page.request.post(`/api/tasks/${task.id}/action-items/${second.id}:complete`, { headers })
  expect(completeSecond.status()).toBe(200)
  await page.getByRole("button", { name: "変更を保存", exact: true }).click()
  const failedActionItem = page.getByRole("group").filter({ hasText: "Delete fails once" })
  await expect(failedActionItem.getByText("Delete fails once: アクションアイテムを保存できませんでした。", { exact: true })).toBeVisible()
  await expect(firstInput).toHaveValue("Update saved once")

  const savedAfterFailure = await page.request.get(`/api/tasks/${task.id}/action-items`, { headers })
  expect(savedAfterFailure.status()).toBe(200)
  const savedRows = (await savedAfterFailure.json()).items
  expect(savedRows.filter((item: { title: string }) => item.title === "Update saved once")).toHaveLength(1)
  expect(savedRows.find((item: { id: string }) => item.id === second.id)).toMatchObject({ id: second.id, completed: true })
  expect(requests.taskCreate).toBe(0)
  expect(requests.firstItemUpdate).toBe(1)
  expect(requests.deleteSecond).toBe(1)
  expect(deleteStatuses).toEqual([409])

  const reopenSecond = await page.request.post(`/api/tasks/${task.id}/action-items/${second.id}:reopen`, { headers })
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
  expect(finalRows.some((item: { id: string }) => item.id === second.id)).toBe(false)
  expect(requests.taskCreate).toBe(0)
  expect(requests.firstItemUpdate).toBe(1)
  expect(requests.deleteSecond).toBe(2)
  expect(deleteStatuses).toEqual([409, 204])
})
