import { expect, test } from "@playwright/test"
import { createAccount, expectAuthenticated, fillCredentials } from "./helpers/auth.ts"

test("only login and signup are public; login persists a session for protected pages", async ({ page, request }) => {
  const credentials = await createAccount(request)

  const projectRequests: string[] = []
  page.on("request", (requestEvent) => {
    const url = new URL(requestEvent.url())
    if (url.pathname.startsWith("/api/projects")) projectRequests.push(url.pathname)
  })
  await page.goto("/projects")
  await expect(page).toHaveURL("/login")
  await expect(page.getByRole("region", { name: "ログイン" })).toBeVisible()
  expect(projectRequests).toHaveLength(0)
  await page.goto("/signup")
  await expect(page).toHaveURL("/signup")
  await expect(page.getByRole("region", { name: "新規登録" })).toBeVisible()
  await page.goto("/login")
  await expect(page).toHaveURL("/login")
  await fillCredentials(page, credentials)
  await page.getByRole("button", { name: "ログイン", exact: true }).click()
  await expectAuthenticated(page, credentials.email)
  await page.goto("/projects")
  await expect(page.getByRole("heading", { name: "プロジェクト", exact: true })).toBeVisible()
  await page.reload()
  await expect(page.getByRole("heading", { name: "プロジェクト", exact: true })).toBeVisible()
})

test("root, deep links, and protected routes redirect before rendering when token is absent", async ({ page }) => {
  const projectRequests: string[] = []
  page.on("request", (requestEvent) => {
    const url = new URL(requestEvent.url())
    if (url.pathname.startsWith("/api/projects")) projectRequests.push(url.pathname)
  })

  for (const path of ["/", "/today", "/projects?status=open", "/projects/new", "/projects/deep-link-id/edit", "/calendar", "/profile", "/not-found"]) {
    await page.goto(path)
    await expect(page).toHaveURL("/login")
    await expect(page.getByRole("region", { name: "ログイン" })).toBeVisible()
  }

  await page.evaluate(() => localStorage.setItem("personal_access_token", "  \t"))
  await page.goto("/projects/deep-link-id/edit")
  await expect(page).toHaveURL("/login")
  expect(projectRequests).toHaveLength(0)
})

test("protected child navigation rechecks the token", async ({ page, request }) => {
  const credentials = await createAccount(request)
  await page.goto("/login")
  await fillCredentials(page, credentials)
  await page.getByRole("button", { name: "ログイン", exact: true }).click()
  await expectAuthenticated(page, credentials.email)

  const projectRequests: string[] = []
  page.on("request", (requestEvent) => {
    const url = new URL(requestEvent.url())
    if (url.pathname.startsWith("/api/projects")) projectRequests.push(url.pathname)
  })
  await page.evaluate(() => localStorage.removeItem("personal_access_token"))
  await page.getByRole("link", { name: "プロジェクト", exact: true }).click()
  await expect(page).toHaveURL("/login")
  await expect(page.getByRole("region", { name: "ログイン" })).toBeVisible()
  expect(projectRequests).toHaveLength(0)
})

test("an incorrect password shows an error and can be corrected", async ({ page, request }) => {
  const credentials = await createAccount(request)
  await page.goto("/login")
  await fillCredentials(page, { ...credentials, password: "Different1!" })
  await page.getByRole("button", { name: "ログイン", exact: true }).click()
  await expect(page.getByText("メールアドレスまたはパスワードが正しくありません。")).toBeVisible()
  expect(await page.evaluate(() => localStorage.getItem("personal_access_token"))).toBeNull()
  await expect(page).toHaveURL("/login")

  await page.getByLabel("パスワード", { exact: true }).fill(credentials.password)
  await page.getByRole("button", { name: "ログイン", exact: true }).click()
  await expectAuthenticated(page, credentials.email)
})
