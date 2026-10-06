import { expect, test } from "vitest"
import { loginSchema } from "./schema"

test.each(["Ab1!", "PASSWORD1!", "password1!", "Password!!", "Password12"])("rejects a password outside the input policy: %s", (password) => {
  const result = loginSchema.safeParse({ email: "test@example.com", password })
  expect(result.success).toBe(false)
  if (!result.success) {
    expect(result.error.issues).toContainEqual(expect.objectContaining({
      path: ["password"], message: "auth.error.invalidPassword",
    }))
  }
})

test("trims email while preserving the password exactly", () => {
  const result = loginSchema.parse({ email: "  test@example.com  ", password: " Password1! " })
  expect(result).toEqual({ email: "test@example.com", password: " Password1! " })
})
