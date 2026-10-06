import { z } from "zod"
import { loginSchema } from "~/features/auth/components/LoginForm/schema"

export const signupSchema = loginSchema.extend({
  passwordConfirmation: z.string().min(1, { error: "auth.error.passwordMismatch" }),
}).refine((data) => data.password === data.passwordConfirmation, {
  error: "auth.error.passwordMismatch",
  path: ["passwordConfirmation"],
})

export type SignupFormValues = z.infer<typeof signupSchema>
