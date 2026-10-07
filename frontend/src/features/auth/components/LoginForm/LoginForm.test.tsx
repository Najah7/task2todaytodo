import { act, cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react"
import { afterEach, beforeEach, expect, test, vi } from "vitest"
import { MemoryRouter, Route, Routes } from "react-router"
import { LanguageProvider } from "~/features/i18n"
import LanguageSwitcher from "~/features/i18n/LanguageSwitcher"
import LoginForm from "./index"

const { login } = vi.hoisted(() => ({ login: vi.fn() }))
vi.mock("~/api/generated/auth", () => ({ usePostLogin: () => ({ mutateAsync: login }) }))

beforeEach(() => {
  login.mockReset()
  localStorage.clear()
  localStorage.setItem("language", "ja")
})
afterEach(cleanup)

function renderForm() {
  render(
    <LanguageProvider>
      <MemoryRouter initialEntries={["/login"]}>
        <LanguageSwitcher />
        <Routes>
          <Route path="/login" element={<LoginForm />} />
          <Route path="/today" element={<h1>Today</h1>} />
        </Routes>
      </MemoryRouter>
    </LanguageProvider>,
  )
}

function fillCredentials() {
  fireEvent.change(screen.getByLabelText("メールアドレス"), { target: { value: "test@example.com" } })
  fireEvent.change(screen.getByLabelText("パスワード", { exact: true }), { target: { value: "Password1!" } })
}

test("field errors follow the selected language and disappear after correction", async () => {
  login.mockResolvedValue({ personal_access_token: "unit-token" })
  renderForm()
  fireEvent.change(screen.getByLabelText("メールアドレス"), { target: { value: "invalid-email" } })
  fireEvent.click(screen.getByRole("button", { name: "ログイン" }))
  await waitFor(() => expect(screen.getAllByRole("alert")).toHaveLength(2))
  expect(login).not.toHaveBeenCalled()
  expect(screen.getByLabelText("メールアドレス").getAttribute("aria-invalid")).toBe("true")

  fireEvent.click(screen.getByRole("switch", { name: "表示言語" }))
  expect(screen.getAllByRole("alert").map((element) => element.textContent)).toEqual([
    "Enter a valid email address.",
    "Use at least 8 characters with an uppercase letter, lowercase letter, number, and symbol.",
  ])
  fireEvent.change(screen.getByLabelText("Email address"), { target: { value: "test@example.com" } })
  fireEvent.change(screen.getByLabelText("Password", { exact: true }), { target: { value: "Password1!" } })
  await waitFor(() => expect(screen.queryAllByRole("alert")).toHaveLength(0))
  fireEvent.click(screen.getByRole("button", { name: "Log in" }))
  expect(await screen.findByRole("heading", { name: "Today" })).toBeTruthy()
})

test("submission stays disabled during a request and recovers after a network failure", async () => {
  let rejectRequest!: (reason: Error) => void
  login.mockReturnValue(new Promise((_, reject) => { rejectRequest = reject }))
  renderForm()
  fillCredentials()
  fireEvent.click(screen.getByRole("button", { name: "ログイン" }))
  expect(await screen.findByRole("button", { name: "ログイン中…" })).toHaveProperty("disabled", true)
  await act(async () => rejectRequest(new TypeError("Failed to fetch")))
  expect(await screen.findByRole("alert")).toHaveProperty("textContent", "サーバーに接続できませんでした。通信環境を確認して再度お試しください。")
  expect(screen.getByRole("button", { name: "ログイン" })).toHaveProperty("disabled", false)
})

test.each(["missing token", "storage failure"])("does not navigate after %s", async (failure) => {
  login.mockResolvedValue(failure === "missing token" ? {} : { personal_access_token: "unit-token" })
  renderForm()
  if (failure === "storage failure") {
    vi.spyOn(Storage.prototype, "setItem").mockImplementation(() => { throw new DOMException("Unavailable", "SecurityError") })
  }
  fillCredentials()
  fireEvent.click(screen.getByRole("button", { name: "ログイン" }))
  const message = failure === "missing token" ? "ログイン情報を取得できませんでした" : "ログイン情報を保存できませんでした"
  expect((await screen.findByRole("alert")).textContent).toContain(message)
  expect(screen.queryByRole("heading", { name: "Today" })).toBeNull()
  expect(localStorage.getItem("personal_access_token")).toBeNull()
})
