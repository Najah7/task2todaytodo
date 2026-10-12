import { expect, test } from "@playwright/test"
import { newCredentials, waitForEmailLink } from "./helpers/auth.ts"

function findHtml(value: unknown): string | undefined {
  if (typeof value === "string") return /<(?:!doctype\s+html|html\b)/i.test(value) ? value : undefined
  if (Array.isArray(value)) {
    for (const item of value) {
      const html = findHtml(item)
      if (html) return html
    }
  } else if (value && typeof value === "object") {
    for (const item of Object.values(value)) {
      const html = findHtml(item)
      if (html) return html
    }
  }
  return undefined
}

test("Mailpit verification email renders Japanese and English CTAs on desktop and mobile", async ({ page, request }, testInfo) => {
  const credentials = newCredentials()
  const signup = await request.post("/api/signup", { data: credentials })
  expect(signup.status()).toBe(202)
  const { url } = await waitForEmailLink(request, credentials.email, "verify-email")

  const mailpitUrl = process.env.E2E_MAILPIT_URL
  if (!mailpitUrl) throw new Error("Mailpit URL was not configured by the E2E runner.")
  const listResponse = await request.get(`${mailpitUrl}/api/v1/messages?limit=100`)
  expect(listResponse.ok()).toBe(true)
  const list = await listResponse.json() as { messages?: { ID?: string; To?: { Address?: string }[] }[] }
  const message = list.messages?.find((item) => item.ID && item.To?.some((to) => to.Address?.toLowerCase() === credentials.email.toLowerCase()))
  expect(message?.ID).toBeTruthy()
  const detailResponse = await request.get(`${mailpitUrl}/api/v1/message/${message?.ID}`)
  expect(detailResponse.ok()).toBe(true)
  const html = findHtml(await detailResponse.json())
  expect(html, "Mailpit message should contain an HTML part").toBeTruthy()
  if (!html) throw new Error("Mailpit message has no HTML part.")

  await page.setContent(html)
  const ctas = page.locator("a")
  await expect(ctas).toHaveCount(2)
  await expect(page.locator("section[lang='ja'] h1")).toHaveText("メールアドレスを確認してください")
  await expect(ctas.nth(0)).toHaveText("日本語で進む")
  await expect(ctas.nth(1)).toHaveText("Continue in English")
  const hrefs = await ctas.evaluateAll((links) => links.map((link) => (link as HTMLAnchorElement).href))
  expect(hrefs[0]).toContain("?lang=ja#token=")
  expect(hrefs[1]).toContain("?lang=en#token=")
  expect(url).toContain("?lang=ja#token=")

  await page.setViewportSize({ width: 1280, height: 900 })
  await page.screenshot({ path: testInfo.outputPath("verification-email-desktop.png"), fullPage: true })
  expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(1280)

  await page.setViewportSize({ width: 375, height: 812 })
  await page.screenshot({ path: testInfo.outputPath("verification-email-mobile.png"), fullPage: true })
  expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(375)
})
