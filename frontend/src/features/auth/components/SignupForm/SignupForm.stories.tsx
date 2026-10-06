import type { Meta, StoryObj } from "@storybook/react-vite"
import { delay, http, HttpResponse } from "msw"
import { expect, userEvent, within } from "storybook/test"
import { withAuthForm } from "~storybook/auth/decorators"
import { authHandlers } from "~storybook/auth/handlers"
import { fillCredentials, submitForm } from "~storybook/auth/interactions"
import SignupForm from "./index"

const meta = {
  title: "Auth/SignupForm",
  component: SignupForm,
  decorators: [withAuthForm],
  parameters: { msw: { handlers: authHandlers } },
} satisfies Meta<typeof SignupForm>

export default meta
type Story = StoryObj<typeof meta>

export const Default: Story = {}

export const PasswordMismatch: Story = {
  play: async ({ canvasElement }) => {
    await fillCredentials(canvasElement)
    await userEvent.type(within(canvasElement).getByLabelText(/^(パスワード（確認）|Confirm password)$/), "Different1!")
    await submitForm(canvasElement)
    await expect(await within(canvasElement).findByRole("alert")).toBeVisible()
  },
}

export const EmailAlreadyExists: Story = {
  parameters: {
    msw: { handlers: { signup: http.post("*/api/signup", () => HttpResponse.json({
      error: { details: [{ code: "email_already_exists" }] },
    }, { status: 409 })) } },
  },
  play: async ({ canvasElement }) => {
    await fillCredentials(canvasElement, true)
    await submitForm(canvasElement)
    await expect(await within(canvasElement).findByRole("alert")).toBeVisible()
  },
}

export const Submitting: Story = {
  parameters: { msw: { handlers: { signup: http.post("*/api/signup", async () => { await delay("infinite") }) } } },
  play: async ({ canvasElement }) => {
    await fillCredentials(canvasElement, true)
    await submitForm(canvasElement)
    await expect(await within(canvasElement).findByRole("button", { name: /^(登録中…|Creating account…)$/ })).toBeDisabled()
  },
}

export const RetryLogin: Story = {
  parameters: { msw: { handlers: { login: http.post("*/api/login", () => HttpResponse.json({}, { status: 503 })) } } },
  play: async ({ canvasElement }) => {
    await fillCredentials(canvasElement, true)
    await submitForm(canvasElement)
    await expect(await within(canvasElement).findByRole("button", { name: /^(ログインして進む|Continue to log in)$/ })).toBeEnabled()
  },
}
