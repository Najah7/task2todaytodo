import { z } from "zod"

export const inviteMemberSchema = z.object({
  email: z.string().trim().pipe(z.email({ error: "auth.error.invalidEmail" })),
  roleId: z.string().min(1),
})

export type InviteMemberValues = z.infer<typeof inviteMemberSchema>
