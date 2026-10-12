import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react"
import { afterEach, beforeAll, describe, expect, it, vi } from "vitest"
import { LanguageProviderContext } from "~/features/i18n/languageContext"
import InviteMemberDialog from "."

afterEach(cleanup)

beforeAll(() => {
  Object.defineProperty(HTMLDialogElement.prototype, "showModal", { configurable: true, value() { this.open = true } })
  Object.defineProperty(HTMLDialogElement.prototype, "close", { configurable: true, value() { this.open = false } })
})

function renderDialog(onSubmit = vi.fn()) {
  const tree = (open: boolean) => (
    <LanguageProviderContext.Provider value={{ language: "ja", setLanguage: vi.fn() }}>
      <InviteMemberDialog open={open} projectName="Project" roles={[{ id: "viewer" }]} onClose={vi.fn()} onSubmit={onSubmit} />
    </LanguageProviderContext.Provider>
  )
  const view = render(tree(false))
  view.rerender(tree(true))
  return onSubmit
}

describe("InviteMemberDialog", () => {
  it("submits a valid ASCII email address", async () => {
    const onSubmit = renderDialog()

    const email = screen.getByRole("textbox") as HTMLInputElement
    fireEvent.change(email, { target: { value: "tuser@example.com" } })
    expect(email.value).toBe("tuser@example.com")
    fireEvent.click(screen.getByRole("button", { name: "招待を送信" }))

    await waitFor(() => expect(onSubmit).toHaveBeenCalledOnce())
    expect(onSubmit.mock.calls[0]?.[0]).toEqual({ email: "tuser@example.com", roleId: "viewer" })
  })

  it("shows a validation error and does not submit a malformed email", async () => {
    const onSubmit = renderDialog()

    fireEvent.change(screen.getByRole("textbox"), { target: { value: "invalid-email" } })
    fireEvent.click(screen.getByRole("button", { name: "招待を送信" }))

    expect((await screen.findByRole("alert")).textContent).toBe("有効なメールアドレスを入力してください。")
    expect(onSubmit).not.toHaveBeenCalled()
  })
})
