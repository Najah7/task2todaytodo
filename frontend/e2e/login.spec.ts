import { expect, test } from "@playwright/test"
import { createAccount, expectAuthenticated, fillCredentials } from "./helpers/auth.ts"

test("login persists a valid session; other pages remain public", async ({ page, request }) => {
  const credentials = await createAccount(request)
  await page.goto("/projects")
  await expect(page.getByRole("heading", { name: "プロジェクト" })).toBeVisible()
  await page.goto("/login")
  await fillCredentials(page, credentials)
  await page.getByRole("button", { name: "ログイン", exact: true }).click()
  await expectAuthenticated(page, credentials.email)
  await page.reload()
  await expectAuthenticated(page, credentials.email)
})

test("an incorrect password shows an error and can be corrected", async ({ page, request }) => {
  const credentials = await createAccount(request)
  await page.goto("/login")
  await fillCredentials(page, { ...credentials, password: "Different1!" })
  await page.getByRole("button", { name: "ログイン", exact: true }).click()
  await expect(page.getByRole("alert")).toHaveText("メールアドレスまたはパスワードが正しくありません。")
  expect(await page.evaluate(() => localStorage.getItem("access_token"))).toBeNull()
  await expect(page).toHaveURL("/login")

  await page.getByLabel("パスワード", { exact: true }).fill(credentials.password)
  await page.getByRole("button", { name: "ログイン", exact: true }).click()
  await expectAuthenticated(page, credentials.email)
})
