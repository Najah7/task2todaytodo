import { z } from "zod"

export const dailyActiveTimeSchema = z.object({
  start: z.string().min(1, "weeklyPlan.activeTime.invalid"),
  end: z.string().min(1, "weeklyPlan.activeTime.invalid"),
  nextDay: z.boolean(),
}).refine((value) => value.nextDay || value.end > value.start, {
  path: ["end"],
  message: "weeklyPlan.activeTime.invalid",
})

export type DailyActiveTimeValues = z.infer<typeof dailyActiveTimeSchema>
