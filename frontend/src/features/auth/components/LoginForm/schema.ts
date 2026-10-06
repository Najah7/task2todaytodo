import { z } from "zod"

const passwordSchema = z.string()
  .min(8, { error: "auth.error.invalidPassword" })
  .regex(/[a-z]/, { error: "auth.error.invalidPassword" })
  .regex(/[A-Z]/, { error: "auth.error.invalidPassword" })
  .regex(/[0-9]/, { error: "auth.error.invalidPassword" })
  .regex(/[!@#$%^&*()_+\-=[\]{};':"\\|,.<>/?]/, { error: "auth.error.invalidPassword" })

export const loginSchema = z.object({
  email: z.string().trim().pipe(z.email({ error: "auth.error.invalidEmail" })),
  password: passwordSchema,
})

export type LoginFormValues = z.infer<typeof loginSchema>
