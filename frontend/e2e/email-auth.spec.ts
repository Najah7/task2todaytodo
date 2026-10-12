import { expect, test } from "@playwright/test"
import { createAccount, fillCredentials, newCredentials, waitForEmailLink } from "./helpers/auth.ts"

test("correct password exposes verification resend; wrong password stays generic", async ({ page, request }) => {
  const credentials = newCredentials()
  const signup = await request.post("/api/signup", { data: credentials })
  expect(signup.status()).toBe(202)

  await page.goto("/login")
  await fillCredentials(page, { ...credentials, password: "WrongPassword1!" })
  await page.getByRole("button", { name: "ログイン", exact: true }).click()
  await expect(page.getByText("メールアドレスまたはパスワードが正しくありません。", { exact: true })).toBeVisible()
  await page.getByLabel("パスワード", { exact: true }).fill(credentials.password)
  await page.getByRole("button", { name: "ログイン", exact: true }).click()
  await expect(page.getByRole("alert")).toContainText("メールアドレスの確認が必要です")
  await page.getByRole("button", { name: "確認メールを再送信", exact: true }).click()
  await expect(page.getByRole("status")).toContainText("対象の場合は確認メールを送信しました")
  expect(await page.evaluate(() => localStorage.getItem("personal_access_token"))).toBeNull()
})

test("password reset link is inert until explicit POST, then new password logs in", async ({ page, request }) => {
  test.setTimeout(120_000)
  const credentials = await createAccount(request)
  const nextPassword = "NewPassword2!"
  await page.goto("/login")
  await fillCredentials(page, credentials)
  await page.getByRole("button", { name: "ログイン", exact: true }).click()
  await expect(page).toHaveURL("/today")
  const oldPersonalAccessToken = await page.evaluate(() => localStorage.getItem("personal_access_token"))
  expect(oldPersonalAccessToken).toBeTruthy()
  if (!oldPersonalAccessToken) throw new Error("Login did not persist a personal access token.")

  await page.goto("/request-password-reset")
  await page.getByLabel("メールアドレス", { exact: true }).fill(credentials.email)
  await page.getByRole("button", { name: "再設定メールを送信", exact: true }).click()
  await expect(page.getByRole("status")).toContainText("対象の場合は再設定メールを送信しました")
  let reset: Awaited<ReturnType<typeof waitForEmailLink>> | undefined
  for (let attempt = 0; attempt < 15 && !reset; attempt += 1) {
    try {
      reset = await waitForEmailLink(request, credentials.email, "reset-password", 1_000)
    } catch {
      const resend = await request.post("/api/password-reset/request", { data: { email: credentials.email } })
      expect(resend.status()).toBe(202)
      if (attempt < 14) await page.waitForTimeout(4_000)
    }
  }
  expect(reset, "password reset mail arrives after the combined cooldown").toBeDefined()
  if (!reset) throw new Error("Password reset mail did not arrive.")
  let confirmationCalls = 0
  page.on("request", (requestEvent) => {
    if (new URL(requestEvent.url()).pathname === "/api/password-reset/confirm") confirmationCalls += 1
  })
  await page.goto(reset.url)
  await expect(page.getByRole("heading", { name: "新しいパスワードを設定", exact: true })).toBeVisible()
  expect(confirmationCalls).toBe(0)
  await page.reload()
  await expect(page.getByRole("heading", { name: "新しいパスワードを設定", exact: true })).toBeVisible()
  await page.getByLabel("パスワード", { exact: true }).fill(nextPassword)
  await page.getByLabel("パスワード（確認）", { exact: true }).fill(nextPassword)
  await page.getByRole("button", { name: "パスワードを更新", exact: true }).click()
  await expect(page).toHaveURL(/\/login\?password-reset=1$/)
  expect(await page.evaluate(() => localStorage.getItem("personal_access_token"))).toBeNull()
  const revokedSession = await request.get("/api/users/me", { headers: { Authorization: `Bearer ${oldPersonalAccessToken}` } })
  expect(revokedSession.status()).toBe(401)
  await page.goBack()
  expect(page.url()).not.toContain(reset.token)
  await page.goto("/login")
  await fillCredentials(page, { ...credentials, password: nextPassword })
  await page.getByRole("button", { name: "ログイン", exact: true }).click()
  await expect(page).toHaveURL("/today")
})

test("same-route fragment changes use the latest token; wrong confirmation password stays on page", async ({ page }) => {
  const submittedBodies: string[] = []
  page.on("request", (requestEvent) => {
    if (new URL(requestEvent.url()).pathname === "/api/email-verification/confirm") {
      submittedBodies.push(requestEvent.postData() ?? "")
    }
  })
  await page.goto("/verify-email?lang=ja#token=first-token")
  await expect(page.getByRole("heading", { name: "メールアドレスを確認", exact: true })).toBeVisible()
  await page.evaluate(() => {
    window.history.pushState(window.history.state, "", "/verify-email?lang=ja#token=second-token")
    window.dispatchEvent(new PopStateEvent("popstate"))
  })
  await page.getByLabel("パスワード", { exact: true }).fill("Password1!")
  await page.getByRole("button", { name: "メールアドレスを確認", exact: true }).click()
  await expect(page.getByRole("alert")).toContainText("確認できませんでした")
  await expect.poll(() => submittedBodies.length).toBe(1)
  expect(JSON.parse(submittedBodies[0])).toMatchObject({ token: "second-token", password: "Password1!" })
  await expect(page).toHaveURL(/#token=second-token$/)
})

test("reset password confirmation must match before request is sent", async ({ page }) => {
  let confirmationCalls = 0
  page.on("request", (requestEvent) => {
    if (new URL(requestEvent.url()).pathname === "/api/password-reset/confirm") confirmationCalls += 1
  })
  await page.goto("/reset-password?lang=ja#token=unused-token")
  await page.getByLabel("パスワード", { exact: true }).fill("Password1!")
  await page.getByLabel("パスワード（確認）", { exact: true }).fill("Different1!")
  await page.getByRole("button", { name: "パスワードを更新", exact: true }).click()
  await expect(page.getByRole("alert")).toHaveText("確認用パスワードが一致していません。")
  expect(confirmationCalls).toBe(0)
})

test("password reset response is identical for eligible and unknown email", async ({ page, request }) => {
  const eligible = await createAccount(request)
  const addresses = [eligible.email, newCredentials().email]
  const responses: string[] = []

  for (const email of addresses) {
    await page.goto("/request-password-reset")
    await page.getByLabel("メールアドレス", { exact: true }).fill(email)
    await page.getByRole("button", { name: "再設定メールを送信", exact: true }).click()
    const guidance = page.getByRole("status")
    await expect(guidance).toBeVisible()
    responses.push((await guidance.textContent()) ?? "")
  }

  expect(responses[1]).toBe(responses[0])
})
