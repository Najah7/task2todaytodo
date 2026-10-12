import { z } from "zod"

export const dailySleepSchema = z.object({
  wake: z.string().min(1, "weeklyPlan.sleep.invalid"),
  bedtime: z.string().min(1, "weeklyPlan.sleep.invalid"),
  nextDay: z.boolean(),
}).refine((value) => value.nextDay || value.bedtime > value.wake, {
  path: ["bedtime"],
  message: "weeklyPlan.sleep.invalid",
})

export type DailySleepValues = z.infer<typeof dailySleepSchema>
