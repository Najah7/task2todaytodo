import { expect, test } from "@playwright/test"

for (const mode of ["light", "dark"] as const) {
  test(`initial display follows the ${mode} OS preference`, async ({ page }) => {
    await page.emulateMedia({ colorScheme: mode })
    await page.goto("/today")
    await expect(page.getByRole("switch", { name: "表示モード" })).toBeChecked({ checked: mode === "dark" })
    await expect(page.locator("html")).toHaveCSS("color-scheme", mode)
  })
}

test("display switches both ways and persists across routes and reloads", async ({ page }) => {
  await page.emulateMedia({ colorScheme: "light" })
  await page.goto("/login")
  await page.getByRole("switch", { name: "表示モード" }).click()
  await expect(page.locator("html")).toHaveCSS("color-scheme", "dark")
  await page.getByRole("link", { name: "今日へ戻る", exact: true }).click()
  await page.reload()
  await expect(page.getByRole("switch", { name: "表示モード" })).toBeChecked()
  await expect(page.locator("html")).toHaveCSS("color-scheme", "dark")
  await page.getByRole("switch", { name: "表示モード" }).click()
  await page.reload()
  await expect(page.getByRole("switch", { name: "表示モード" })).not.toBeChecked()
  await expect(page.locator("html")).toHaveCSS("color-scheme", "light")
})
