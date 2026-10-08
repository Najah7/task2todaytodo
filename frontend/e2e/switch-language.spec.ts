import { expect, test } from "@playwright/test"

test.describe("English browser", () => {
  test.use({ locale: "en-US" })

  test("initial language follows the browser setting", async ({ page }) => {
    await page.goto("/login")
    await expect(page.getByRole("heading", { name: "Log in", exact: true })).toBeVisible()
    await expect(page.getByRole("switch", { name: "Display language" })).not.toBeChecked()
  })
})

test("language switches both ways and persists across routes and reloads", async ({ page }) => {
  await page.goto("/login")
  await expect(page.getByRole("heading", { name: "ログイン", exact: true })).toBeVisible()
  await page.getByRole("switch", { name: "表示言語" }).click()
  await expect(page.getByRole("heading", { name: "Log in", exact: true })).toBeVisible()
  await expect(page.getByRole("link", { name: "Sign up free", exact: true })).toBeVisible()
  await expect(page.locator("html")).toHaveAttribute("lang", "en")
  await page.getByRole("link", { name: "Sign up free", exact: true }).click()
  await expect(page).toHaveURL("/signup")
  await page.reload()
  await expect(page.getByRole("heading", { name: "Create account", exact: true })).toBeVisible()
  await page.getByRole("switch", { name: "Display language" }).click()
  await page.reload()
  await expect(page.getByRole("heading", { name: "新規登録", exact: true })).toBeVisible()
  await expect(page.locator("html")).toHaveAttribute("lang", "ja")
})
