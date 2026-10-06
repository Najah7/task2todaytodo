import type { Meta, StoryObj } from "@storybook/react-vite"
import { delay, http, HttpResponse } from "msw"
import { expect, within } from "storybook/test"
import { withAuthForm } from "~storybook/auth/decorators"
import { authHandlers } from "~storybook/auth/handlers"
import { fillCredentials, submitForm } from "~storybook/auth/interactions"
import LoginForm from "./index"

const meta = {
  title: "Auth/LoginForm",
  component: LoginForm,
  decorators: [withAuthForm],
  parameters: { msw: { handlers: authHandlers } },
} satisfies Meta<typeof LoginForm>

export default meta
type Story = StoryObj<typeof meta>

export const Default: Story = {}

export const ValidationErrors: Story = {
  play: async ({ canvasElement }) => {
    await submitForm(canvasElement)
    await expect(await within(canvasElement).findAllByRole("alert")).toHaveLength(2)
  },
}

export const InvalidCredentials: Story = {
  parameters: {
    msw: { handlers: { login: http.post("*/api/login", () => HttpResponse.json({
      error: { details: [{ code: "invalid_credentials" }] },
    }, { status: 401 })) } },
  },
  play: async ({ canvasElement }) => {
    await fillCredentials(canvasElement)
    await submitForm(canvasElement)
    await expect(await within(canvasElement).findByRole("alert")).toBeVisible()
  },
}

export const Submitting: Story = {
  parameters: { msw: { handlers: { login: http.post("*/api/login", async () => { await delay("infinite") }) } } },
  play: async ({ canvasElement }) => {
    await fillCredentials(canvasElement)
    await submitForm(canvasElement)
    await expect(await within(canvasElement).findByRole("button", { name: /^(ログイン中…|Logging in…)$/ })).toBeDisabled()
  },
}
