import { expect, test } from "@playwright/test"

test.describe("English browser", () => {
  test.use({ locale: "en-US" })

  test("initial language follows the browser setting", async ({ page }) => {
    await page.goto("/today")
    await expect(page.getByRole("heading", { name: "Today", exact: true })).toBeVisible()
    await expect(page.getByRole("switch", { name: "Display language" })).not.toBeChecked()
  })
})

test("language switches both ways and persists across routes and reloads", async ({ page }) => {
  await page.goto("/today")
  await expect(page.getByRole("heading", { name: "今日のタスク", exact: true })).toBeVisible()
  await page.getByRole("switch", { name: "表示言語" }).click()
  await expect(page.getByRole("heading", { name: "Today", exact: true })).toBeVisible()
  await expect(page.getByRole("link", { name: "Task Inbox", exact: true })).toBeVisible()
  await expect(page.locator("html")).toHaveAttribute("lang", "en")
  await page.getByRole("link", { name: "Projects", exact: true }).click()
  await page.reload()
  await expect(page.getByRole("heading", { name: "Projects", exact: true })).toBeVisible()
  await page.getByRole("switch", { name: "Display language" }).click()
  await page.reload()
  await expect(page.getByRole("heading", { name: "プロジェクト", exact: true })).toBeVisible()
  await expect(page.locator("html")).toHaveAttribute("lang", "ja")
})
