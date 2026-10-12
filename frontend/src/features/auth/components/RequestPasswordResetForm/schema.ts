import { z } from "zod"
import { loginSchema } from "~/features/auth/components/LoginForm/schema"

export const requestPasswordResetSchema = z.object({ email: loginSchema.shape.email })
export type RequestPasswordResetFormValues = z.infer<typeof requestPasswordResetSchema>
