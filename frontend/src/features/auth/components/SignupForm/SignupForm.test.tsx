import { cleanup, fireEvent, render, screen } from "@testing-library/react"
import { afterEach, beforeEach, expect, test, vi } from "vitest"
import { MemoryRouter, Route, Routes } from "react-router"
import { ApiError } from "~/api/http"
import { LanguageProvider } from "~/features/i18n"
import SignupForm from "./index"

const { signup, login } = vi.hoisted(() => ({ signup: vi.fn(), login: vi.fn() }))
vi.mock("~/api/generated/auth", () => ({
  usePostSignup: () => ({ mutateAsync: signup }),
  usePostLogin: () => ({ mutateAsync: login }),
}))

beforeEach(() => {
  signup.mockReset()
  login.mockReset()
  localStorage.clear()
  localStorage.setItem("language", "ja")
})
afterEach(cleanup)

function renderForm() {
  render(
    <LanguageProvider>
      <MemoryRouter initialEntries={["/signup"]}>
        <Routes>
          <Route path="/signup" element={<SignupForm />} />
          <Route path="/today" element={<h1>Today</h1>} />
        </Routes>
      </MemoryRouter>
    </LanguageProvider>,
  )
  fireEvent.change(screen.getByLabelText("メールアドレス"), { target: { value: "test@example.com" } })
  for (const label of ["パスワード", "パスワード（確認）"]) {
    fireEvent.change(screen.getByLabelText(label, { exact: true }), { target: { value: "Password1!" } })
  }
}

test("retries only login after the account has been created", async () => {
  signup.mockResolvedValue({ user_id: "unit-user" })
  login.mockRejectedValueOnce(new ApiError(503, {})).mockResolvedValueOnce({ token: "unit-token" })
  renderForm()
  fireEvent.click(screen.getByRole("button", { name: "アカウントを作成" }))
  expect((await screen.findByRole("alert")).textContent).toContain("登録は完了しました")
  expect(screen.getByLabelText("メールアドレス")).toHaveProperty("readOnly", true)
  expect(screen.getByLabelText("パスワード", { exact: true })).toHaveProperty("readOnly", true)
  fireEvent.click(screen.getByRole("button", { name: "ログインして進む" }))
  expect(await screen.findByRole("heading", { name: "Today" })).toBeTruthy()
  expect(signup).toHaveBeenCalledExactlyOnceWith({ data: { email: "test@example.com", password: "Password1!" } })
  expect(login).toHaveBeenCalledTimes(2)
})

test("shows backend validation failures without creating a session", async () => {
  signup.mockRejectedValue(new ApiError(400, { error: { details: [{ code: "invalid_password" }] } }))
  renderForm()
  fireEvent.click(screen.getByRole("button", { name: "アカウントを作成" }))
  expect(await screen.findByRole("alert")).toHaveProperty("textContent", "パスワードは8文字以上で、英大文字・英小文字・数字・記号を含めてください。")
  expect(login).not.toHaveBeenCalled()
  expect(localStorage.getItem("access_token")).toBeNull()
  expect(screen.getByRole("button", { name: "アカウントを作成" })).toHaveProperty("disabled", false)
})
