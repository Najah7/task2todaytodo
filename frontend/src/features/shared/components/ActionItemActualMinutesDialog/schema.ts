import { z } from "zod"

export const actionItemActualMinutesSchema = z.object({
  actualMinutes: z.string().trim().refine((value) => {
    if (value === "") return true
    const minutes = Number(value)
    return /^\d+$/.test(value) && Number.isSafeInteger(minutes) && minutes >= 0 && minutes <= 2_147_483_647
  }, "actualMinutes.invalid"),
})

export type ActionItemActualMinutesFormValues = z.infer<typeof actionItemActualMinutesSchema>
