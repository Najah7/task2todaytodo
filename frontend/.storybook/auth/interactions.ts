import { userEvent, within } from "storybook/test"

export async function fillCredentials(canvasElement: HTMLElement, signup = false) {
  const canvas = within(canvasElement)
  await userEvent.type(canvas.getByLabelText(/^(メールアドレス|Email address)$/), "member@example.com")
  await userEvent.type(canvas.getByLabelText(/^(パスワード|Password)$/), "Password1!")
  if (signup) await userEvent.type(canvas.getByLabelText(/^(パスワード（確認）|Confirm password)$/), "Password1!")
}

export async function submitForm(canvasElement: HTMLElement) {
  await userEvent.click(within(canvasElement).getByRole("button", {
    name: /^(ログイン|Log in|アカウントを作成|Create account)$/,
  }))
}
