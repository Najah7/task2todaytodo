import { z } from "zod"
import { loginSchema } from "~/features/auth/components/LoginForm/schema"

export const verifyEmailSchema = z.object({ password: loginSchema.shape.password })
export type VerifyEmailFormValues = z.infer<typeof verifyEmailSchema>
