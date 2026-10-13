import { cleanup, fireEvent, render, screen } from "@testing-library/react"
import { afterEach, beforeEach, expect, test, vi } from "vitest"
import { QueryClient, QueryClientProvider } from "@tanstack/react-query"
import { MemoryRouter, Route, Routes } from "react-router"
import { ApiError } from "~/api/http"
import { LanguageProvider } from "~/features/i18n"
import { NotificationViewport } from "~/features/shared/notification"
import SignupForm from "./index"

const { signup } = vi.hoisted(() => ({ signup: vi.fn() }))
vi.mock("~/api/generated/auth", () => ({
  usePostSignup: () => ({ mutateAsync: signup }),
}))

beforeEach(() => {
  signup.mockReset()
  localStorage.clear()
  localStorage.setItem("language", "ja")
})
afterEach(cleanup)

function renderForm() {
  render(
    <QueryClientProvider client={new QueryClient()}>
      <LanguageProvider>
        <MemoryRouter initialEntries={["/signup"]}>
          <Routes>
            <Route path="/signup" element={<SignupForm />} />
            <Route path="/inbox" element={<h1>Inbox</h1>} />
          </Routes>
          <NotificationViewport />
        </MemoryRouter>
      </LanguageProvider>
    </QueryClientProvider>,
  )
  fireEvent.change(screen.getByLabelText("メールアドレス"), { target: { value: "test@example.com" } })
  for (const label of ["パスワード", "パスワード（確認）"]) {
    fireEvent.change(screen.getByLabelText(label, { exact: true }), { target: { value: "Password1!" } })
  }
}

test("shows generic verification guidance without creating a session", async () => {
  signup.mockResolvedValue({ message: "Request received" })
  renderForm()
  fireEvent.click(screen.getByRole("button", { name: "アカウントを作成" }))
  expect((await screen.findByRole("status")).textContent).toContain("確認メールを送信しました")
  expect(screen.getByRole("button", { name: "確認メールを再送信" })).toBeTruthy()
  expect(signup).toHaveBeenCalledExactlyOnceWith({ data: { email: "test@example.com", password: "Password1!" } })
  expect(localStorage.getItem("personal_access_token")).toBeNull()
})

test("notifies backend validation failures without creating a session", async () => {
  signup.mockRejectedValue(new ApiError(400, { error: { details: [{ code: "invalid_password" }] } }))
  renderForm()
  fireEvent.click(screen.getByRole("button", { name: "アカウントを作成" }))
  expect(await screen.findByText("パスワードは8文字以上で、英大文字・英小文字・数字・記号を含めてください。")).toBeTruthy()
  expect(localStorage.getItem("personal_access_token")).toBeNull()
  expect(screen.getByRole("button", { name: "アカウントを作成" })).toHaveProperty("disabled", false)
})
