import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react"
import { afterEach, expect, test, vi } from "vitest"
import { LanguageProviderContext } from "~/features/i18n/languageContext"
import ActionItemActualMinutesDialog from "."
import { actionItemActualMinutesSchema } from "./schema"

afterEach(cleanup)

function renderDialog(onComplete = vi.fn()) {
  render(<LanguageProviderContext.Provider value={{ language: "ja", setLanguage: vi.fn() }}><ActionItemActualMinutesDialog open onComplete={onComplete} onCancel={vi.fn()} /></LanguageProviderContext.Provider>)
  return onComplete
}

test("skip completes without actual minutes", () => {
  const onComplete = renderDialog()
  fireEvent.click(screen.getByRole("button", { name: "入力せずに完了" }))
  expect(onComplete).toHaveBeenCalledWith()
})

test("accepts zero as a valid actual duration", async () => {
  const onComplete = renderDialog()
  const input = screen.getByLabelText("実績時間（分）") as HTMLInputElement
  fireEvent.change(input, { target: { value: "0" } })
  expect(input.value).toBe("0")
  fireEvent.click(screen.getByRole("button", { name: "実績を保存して完了" }))
  await waitFor(() => expect(onComplete).toHaveBeenCalledWith(0))
})

test("shows an inline error for an invalid actual duration", async () => {
  const onComplete = renderDialog()
  fireEvent.change(screen.getByLabelText("実績時間（分）"), { target: { value: "-1" } })
  fireEvent.click(screen.getByRole("button", { name: "実績を保存して完了" }))
  expect((await screen.findByRole("alert")).textContent).toBe("0以上の整数を入力してください。")
  expect(onComplete).not.toHaveBeenCalled()
})

test("matches the API integer range", () => {
  expect(actionItemActualMinutesSchema.safeParse({ actualMinutes: "2147483647" }).success).toBe(true)
  expect(actionItemActualMinutesSchema.safeParse({ actualMinutes: "2147483648" }).success).toBe(false)
})
