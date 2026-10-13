import { z } from "zod"

export const minimumGapSchema = z.object({
  minimumGapMinutes: z.number().int().min(0, "profile.minimumGap.invalid").max(1440, "profile.minimumGap.invalid"),
})

export type MinimumGapValues = z.infer<typeof minimumGapSchema>
