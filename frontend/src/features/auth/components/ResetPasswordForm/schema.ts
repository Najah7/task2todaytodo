import { z } from "zod"
import { loginSchema } from "~/features/auth/components/LoginForm/schema"

export const resetPasswordSchema = z.object({
  password: loginSchema.shape.password,
  passwordConfirmation: z.string(),
}).refine((values) => values.password === values.passwordConfirmation, {
  path: ["passwordConfirmation"],
  error: "auth.error.passwordMismatch",
})
export type ResetPasswordFormValues = z.infer<typeof resetPasswordSchema>
