import { z } from "zod"
import { loginSchema } from "~/features/auth/components/LoginForm/schema"

export const invitationLoginSchema = loginSchema.pick({ password: true })
export const invitationNewAccountSchema = z.object({
  password: loginSchema.shape.password,
  passwordConfirmation: z.string().min(1, { error: "auth.error.passwordMismatch" }),
}).refine((data) => data.password === data.passwordConfirmation, {
  error: "auth.error.passwordMismatch",
  path: ["passwordConfirmation"],
})

export type InvitationLoginValues = z.infer<typeof invitationLoginSchema>
export type InvitationNewAccountValues = z.infer<typeof invitationNewAccountSchema>
