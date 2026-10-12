import { mkdir } from "node:fs/promises"
import { resolve } from "node:path"
import { expect, test } from "@playwright/test"
import { createAccount, fillCredentials, newCredentials, waitForEmailLink } from "./helpers/auth.ts"

test("new invited member sets a password and accepts only after explicit submit", async ({ page, request }) => {
  test.setTimeout(120_000)
  await page.setViewportSize({ width: 1440, height: 900 })
  await mkdir(resolve("test-results/members-visual"), { recursive: true })
  const owner = await createAccount(request)
  await page.goto("/login")
  await fillCredentials(page, owner)
  await page.getByRole("button", { name: "ログイン", exact: true }).click()
  await expect(page).toHaveURL("/today")

  const ownerToken = await page.evaluate(() => localStorage.getItem("personal_access_token"))
  expect(ownerToken).toBeTruthy()
  const headers = { Authorization: `Bearer ${ownerToken}` }
  const projectResponse = await request.post("/api/projects", {
    headers,
    data: { title: "Invitation project", goal: "Share work", description: "", type: "other", priority: "low", start_date: null, end_date: null },
  })
  expect(projectResponse.status()).toBe(201)
  const project = await projectResponse.json() as { id: string }
  const invited = newCredentials()
  const invitationResponse = await request.post(`/api/projects/${project.id}/invitations`, {
    headers,
    data: { email: invited.email, role_id: "viewer" },
  })
  expect(invitationResponse.status()).toBe(202)
  const invitationLink = await waitForEmailLink(request, invited.email, "invitations")

  await page.evaluate(() => localStorage.removeItem("personal_access_token"))
  let acceptCalls = 0
  page.on("request", (event) => {
    if (new URL(event.url()).pathname === "/api/invitations/accept") acceptCalls += 1
  })
  await page.goto(invitationLink.url)
  await expect(page.getByRole("heading", { name: "プロジェクトへの招待", exact: true })).toBeVisible()
  await expect(page.getByText(`招待先: ${invited.email}`, { exact: true })).toBeVisible()
  expect(acceptCalls).toBe(0)
  await page.reload()
  await expect(page.getByText(`招待先: ${invited.email}`, { exact: true })).toBeVisible()
  expect(acceptCalls).toBe(0)

  await page.getByLabel("パスワード", { exact: true }).fill(invited.password)
  await page.getByLabel("パスワード（確認）", { exact: true }).fill(invited.password)
  await page.getByRole("button", { name: "招待を受け入れる", exact: true }).click()
  await expect(page).toHaveURL((url) => url.pathname === "/members" && url.searchParams.get("projectId") === project.id)
  await expect(page.getByRole("heading", { name: "メンバー", exact: true })).toBeVisible()
  await expect(page.getByText(invited.email, { exact: true })).toBeVisible()
  await page.screenshot({ path: resolve("test-results/members-visual/desktop.png"), fullPage: true })
  await page.setViewportSize({ width: 390, height: 844 })
  await page.screenshot({ path: resolve("test-results/members-visual/mobile.png"), fullPage: true })
  expect(acceptCalls).toBe(1)
  await expect.poll(async () => {
    const token = await page.evaluate(() => localStorage.getItem("personal_access_token"))
    return token ? (await page.request.get("/api/users/me", { headers: { Authorization: `Bearer ${token}` } })).status() : null
  }).toBe(200)
})
