import { randomUUID } from "node:crypto"
import { expect, type APIRequestContext, type Page } from "@playwright/test"

export function newCredentials() {
  return { email: `e2e-${randomUUID()}@example.test`, password: "Password1!" }
}

export async function createAccount(request: APIRequestContext) {
  const credentials = newCredentials()
  const response = await request.post("/api/signup", { data: credentials })
  expect(response.status()).toBe(201)
  return credentials
}

export async function fillCredentials(page: Page, credentials: ReturnType<typeof newCredentials>, signup = false) {
  await page.getByLabel("メールアドレス", { exact: true }).fill(credentials.email)
  await page.getByLabel("パスワード", { exact: true }).fill(credentials.password)
  if (signup) await page.getByLabel("パスワード（確認）", { exact: true }).fill(credentials.password)
}

export async function expectAuthenticated(page: Page, email: string) {
  await expect(page).toHaveURL("/today")
  const token = await page.evaluate(() => localStorage.getItem("access_token"))
  expect(token).toBeTruthy()
  const response = await page.request.get("/api/users/me", { headers: { Authorization: `Bearer ${token}` } })
  expect(response.status()).toBe(200)
  expect(await response.json()).toMatchObject({ email })
}
