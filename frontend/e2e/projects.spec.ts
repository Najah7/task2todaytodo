import { mkdir } from "node:fs/promises"
import { resolve } from "node:path"
import { expect, test } from "@playwright/test"
import { createAccount, fillCredentials } from "./helpers/auth.ts"

async function signIn(page: import("@playwright/test").Page, request: import("@playwright/test").APIRequestContext) {
  const credentials = await createAccount(request)
  await page.goto("/login")
  await fillCredentials(page, credentials)
  await page.getByRole("button", { name: "ログイン", exact: true }).click()
  await expect(page).toHaveURL("/today")
  return credentials
}

test("creates, changes status, trashes, restores, and edits a project", async ({ page, request }) => {
  await page.setViewportSize({ width: 1440, height: 1000 })
  await mkdir(resolve("test-results/projects-visuals"), { recursive: true })
  await signIn(page, request)
  await page.goto("/projects?status=open")
  await expect(page.getByRole("heading", { name: "プロジェクト", exact: true })).toBeVisible()
  await page.getByRole("button", { name: "プロジェクトを作成", exact: true }).click()
  const typeSelect = page.getByRole("button", { name: /^種類 / })
  await typeSelect.click()
  const typeSearch = page.getByRole("combobox", { name: "種類" })
  await expect(typeSearch).toBeVisible()
  await page.screenshot({ path: resolve("test-results/projects-visuals/project-form-select-light.png"), fullPage: true })
  await typeSearch.press("Tab")
  const prioritySelect = page.getByRole("button", { name: /^優先度 / })
  await expect(prioritySelect).toBeFocused()
  await prioritySelect.click()
  await page.getByRole("combobox", { name: "優先度" }).press("Shift+Tab")
  await expect(typeSelect).toBeFocused()
  const themeSwitch = page.getByRole("switch", { name: "表示モード" })
  await themeSwitch.click()
  await expect(themeSwitch).toHaveAttribute("aria-checked", "true")
  await typeSelect.click()
  await expect(page.getByRole("combobox", { name: "種類" })).toBeVisible()
  await page.screenshot({ path: resolve("test-results/projects-visuals/project-form-select-dark.png"), fullPage: true })
  await page.getByRole("combobox", { name: "種類" }).press("Escape")
  await themeSwitch.click()
  await expect(themeSwitch).toHaveAttribute("aria-checked", "false")
  await page.getByLabel("プロジェクト名").fill("Integration plan")
  await page.getByLabel("ゴール（WHY）").fill("A real API project")
  await page.getByLabel("詳細").fill("Created by the isolated E2E backend")
  await page.getByRole("button", { name: "作成する", exact: true }).click()
  await expect(page.getByText("プロジェクトを作成しました。")).toBeVisible()
  await expect(page).toHaveURL(/\/projects\?status=open/)
  await expect(page.getByText("Integration plan", { exact: true })).toBeVisible()

  const statusSelect = page.getByRole("button", { name: /^Integration planの状態/ })
  await statusSelect.click()
  const statusSearch = page.getByRole("combobox", { name: "Integration planの状態" })
  await expect(statusSearch).toBeVisible()
  const popupBounds = await statusSearch.locator("xpath=../..").boundingBox()
  expect(popupBounds).not.toBeNull()
  if (popupBounds) {
    const viewport = page.viewportSize()
    expect(popupBounds.x).toBeGreaterThanOrEqual(0)
    expect(popupBounds.y).toBeGreaterThanOrEqual(0)
    expect(popupBounds.x + popupBounds.width).toBeLessThanOrEqual(viewport?.width ?? 0)
    expect(popupBounds.y + popupBounds.height).toBeLessThanOrEqual(viewport?.height ?? 0)
  }
  await page.screenshot({ path: resolve("test-results/projects-visuals/status-search-popup.png"), fullPage: true })
  await statusSearch.fill("留")
  await expect(page.getByRole("option")).toHaveCount(1)
  await page.screenshot({ path: resolve("test-results/projects-visuals/status-search-filter.png"), fullPage: true })
  await page.getByRole("option", { name: "保留", exact: true }).click()
  await expect(page.getByText("この状態のプロジェクトはありません。", { exact: true })).toBeVisible()
  await page.getByRole("button", { name: /保留/ }).click()
  await expect(page.getByText("Integration plan", { exact: true })).toBeVisible()

  const trashButton = page.getByRole("button", { name: "Integration planをゴミ箱に移動" })
  await trashButton.click()
  const trashDialog = page.getByRole("alertdialog", { name: "プロジェクトをゴミ箱に移動しますか？" })
  await expect(trashDialog.getByText(/通常の一覧から非表示/)).toBeVisible()
  await page.screenshot({ path: resolve("test-results/projects-visuals/trash-confirmation.png"), fullPage: true })
  await trashDialog.getByRole("button", { name: "Integration planをゴミ箱に移動" }).click()
  await expect(page.getByText("この状態のプロジェクトはありません。", { exact: true })).toBeVisible()
  await page.getByRole("button", { name: /ゴミ箱/ }).click()
  await expect(page.getByText("Integration plan", { exact: true })).toBeVisible()

  await themeSwitch.click()
  await expect(themeSwitch).toHaveAttribute("aria-checked", "true")
  await page.screenshot({ path: resolve("test-results/projects-visuals/list-dark-trash.png"), fullPage: true })
  await themeSwitch.click()
  await expect(themeSwitch).toHaveAttribute("aria-checked", "false")

  await page.getByRole("button", { name: "復元", exact: true }).click()
  await expect(page).toHaveURL(/\/projects\?status=pending/)
  await expect(page.getByText("Integration plan", { exact: true })).toBeVisible()
  await page.screenshot({ path: resolve("test-results/projects-visuals/list-light.png"), fullPage: true })
  await page.getByRole("button", { name: "編集", exact: true }).click()
  await expect(page).toHaveURL(/\/projects\/.*\/edit/)
  await page.getByLabel("詳細").fill("Unsaved draft stays until confirmed")
  await page.getByLabel("開始日").fill("2026-10-20")
  await page.getByLabel("期限").fill("2026-10-10")
  await page.getByRole("button", { name: "保存する", exact: true }).click()
  await expect(page.getByText("期限は開始日以降の日付を入力してください。", { exact: true })).toBeVisible()
  await page.evaluate(() => {
    if (document.activeElement instanceof HTMLElement) document.activeElement.blur()
    window.scrollTo(0, 0)
  })
  await page.screenshot({ path: resolve("test-results/projects-visuals/form-error.png"), fullPage: true })
  await page.getByLabel("期限").fill("")
  await page.getByRole("navigation", { name: "パンくずリスト" }).getByRole("link", { name: "プロジェクト", exact: true }).click()
  const discardDialog = page.getByRole("alertdialog", { name: "変更を破棄しますか？" })
  await expect(discardDialog).toBeVisible()
  await page.screenshot({ path: resolve("test-results/projects-visuals/dirty-confirmation.png"), fullPage: true })
  await discardDialog.getByRole("button", { name: "編集を続ける", exact: true }).click()
  await expect(discardDialog).toBeHidden()
  await expect(page.getByLabel("詳細")).toHaveValue("Unsaved draft stays until confirmed")
  await expect(page.getByLabel("詳細")).toBeEnabled()
  await page.getByLabel("詳細").click()
  await page.getByLabel("詳細").fill("Saved integration note")
  await expect(page.getByLabel("詳細")).toHaveValue("Saved integration note")
  await page.getByRole("button", { name: "保存する", exact: true }).click()
  await expect(page.getByText("プロジェクトを保存しました。")).toBeVisible()
  await expect(page).toHaveURL(/\/projects\?status=pending/)
  await page.getByRole("button", { name: "編集", exact: true }).click()
  const projectId = new URL(page.url()).pathname.split("/").at(-2)
  const token = await page.evaluate(() => localStorage.getItem("personal_access_token"))
  const savedProject = await page.request.get(`/api/projects/${projectId}`, {
    headers: { Authorization: `Bearer ${token}` },
  })
  expect(savedProject.status()).toBe(200)
  expect((await savedProject.json()).description).toBe("Saved integration note")
  await expect(page.getByLabel("詳細")).toHaveValue("Saved integration note")
})

test("preserves an edit draft through revision conflicts until explicit reload", async ({ page, request }) => {
  await signIn(page, request)
  const token = await page.evaluate(() => localStorage.getItem("personal_access_token"))
  expect(token).toBeTruthy()
  const headers = { Authorization: `Bearer ${token}` }
  const create = await page.request.post("/api/projects", {
    headers,
    data: { title: "Shared project", goal: "Remote baseline", description: "", type: "other", priority: "low", start_date: null, end_date: null },
  })
  expect(create.status()).toBe(201)
  const project = await create.json() as { id: string; revision: number }

  await page.goto(`/projects/${project.id}/edit`)
  await expect(page.getByLabel("プロジェクト名")).toHaveValue("Shared project")
  await page.getByLabel("ゴール（WHY）").fill("My first draft")
  const remoteUpdate = await page.request.patch(`/api/projects/${project.id}`, {
    headers: { ...headers, "If-Match": `"${project.revision}"` },
    data: { title: "Updated elsewhere" },
  })
  expect(remoteUpdate.status()).toBe(200)

  await page.getByRole("button", { name: "保存する", exact: true }).click()
  await expect(page.getByText("プロジェクトが更新されています。最新の内容を確認してください。").first()).toBeVisible()
  await expect(page.getByLabel("ゴール（WHY）")).toHaveValue("My first draft")
  await page.getByRole("button", { name: "最新の内容を読み込む", exact: true }).click()
  const reloadDialog = page.getByRole("alertdialog", { name: "最新の内容を読み込みますか？" })
  await reloadDialog.getByRole("button", { name: "最新の内容を読み込む", exact: true }).click()
  await expect(page.getByLabel("プロジェクト名")).toHaveValue("Updated elsewhere")
  await expect(page.getByLabel("ゴール（WHY）")).toHaveValue("Remote baseline")
})

test("clears the previous account project cache after signing in as another user", async ({ page, request }) => {
  const firstCredentials = await signIn(page, request)
  const firstToken = await page.evaluate(() => localStorage.getItem("personal_access_token"))
  expect(firstToken).toBeTruthy()
  const created = await page.request.post("/api/projects", {
    headers: { Authorization: `Bearer ${firstToken}` },
    data: { title: "Only account A project", goal: "Private", description: "", type: "other", priority: "low", start_date: null, end_date: null },
  })
  expect(created.status()).toBe(201)
  await page.goto("/projects?status=open")
  await expect(page.getByText("Only account A project", { exact: true })).toBeVisible()

  const secondCredentials = await createAccount(request)
  await page.evaluate(() => localStorage.removeItem("personal_access_token"))
  await page.goto("/login")
  await fillCredentials(page, secondCredentials)
  await page.getByRole("button", { name: "ログイン", exact: true }).click()
  await expect(page).toHaveURL("/today")

  let unblock: (() => void) | undefined
  let markRequestStarted: (() => void) | undefined
  const holdRequest = new Promise<void>((resolve) => { unblock = resolve })
  const requestStarted = new Promise<void>((resolve) => { markRequestStarted = resolve })
  await page.route((url) => url.pathname === "/api/projects", async (route) => {
    markRequestStarted?.()
    await holdRequest
    await route.continue()
  })
  try {
    const navigation = page.goto("/projects?status=open")
    await requestStarted
    await expect(page.getByText("Only account A project", { exact: true })).toHaveCount(0)
    unblock?.()
    await navigation
  } finally {
    unblock?.()
    await page.unroute((url) => url.pathname === "/api/projects")
  }
  await expect(page.getByText("この状態のプロジェクトはありません。", { exact: true })).toBeVisible()
  expect(firstCredentials.email).not.toBe(secondCredentials.email)
})
