import { randomUUID } from "node:crypto"
import { expect, type APIRequestContext, type Page } from "@playwright/test"

export function newCredentials() {
  return { email: `e2e-${randomUUID()}@example.test`, password: "Password1!" }
}

export async function waitForEmailLink(request: APIRequestContext, email: string, route: string, timeoutMs = 10_000) {
  const mailpitUrl = process.env.E2E_MAILPIT_URL
  if (!mailpitUrl) throw new Error("Mailpit URL was not configured by the E2E runner.")
  for (let attempt = 0; attempt < Math.ceil(timeoutMs / 250); attempt += 1) {
    const listResponse = await request.get(`${mailpitUrl}/api/v1/messages?limit=100`)
    if (listResponse.ok()) {
      const list = await listResponse.json() as { messages?: { ID?: string; To?: { Address?: string }[] }[] }
      const messages = list.messages?.filter((item) => item.To?.some((recipient) => recipient.Address?.toLowerCase() === email.toLowerCase())) ?? []
      for (const message of messages) {
        if (!message.ID) continue
        const detailResponse = await request.get(`${mailpitUrl}/api/v1/message/${message.ID}`)
        if (detailResponse.ok()) {
          const content = JSON.stringify(await detailResponse.json())
          const url = content.match(/https?:[^\\\s"<>]+/g)?.map((value) => value.replace(/\\\\u0026/g, "&").replace(/&amp;/g, "&")).find((value) => value.includes(`/${route}?`))
          const token = url && new URL(url).hash.match(/(?:^#|&)token=([^&]+)/)?.[1]
          if (url && token) return { url, token }
        }
      }
    }
    await new Promise((resolve) => setTimeout(resolve, 250))
  }
  throw new Error(`Mailpit did not receive ${route} email for ${email}.`)
}

export async function createAccount(request: APIRequestContext) {
  const credentials = newCredentials()
  const response = await request.post("/api/signup", { data: credentials })
  expect(response.status()).toBe(202)
  const { token } = await waitForEmailLink(request, credentials.email, "verify-email")
  const confirmation = await request.post("/api/email-verification/confirm", {
    data: { token, password: credentials.password },
  })
  expect(confirmation.ok()).toBe(true)
  return credentials
}

export async function fillCredentials(page: Page, credentials: ReturnType<typeof newCredentials>, signup = false) {
  await page.getByLabel("メールアドレス", { exact: true }).fill(credentials.email)
  await page.getByLabel("パスワード", { exact: true }).fill(credentials.password)
  if (signup) await page.getByLabel("パスワード（確認）", { exact: true }).fill(credentials.password)
}

export async function expectAuthenticated(page: Page, email: string) {
  await expect(page).toHaveURL("/inbox")
  const token = await page.evaluate(() => localStorage.getItem("personal_access_token"))
  expect(token).toBeTruthy()
  const response = await page.request.get("/api/users/me", { headers: { Authorization: `Bearer ${token}` } })
  expect(response.status()).toBe(200)
  expect(await response.json()).toMatchObject({ email })
}
