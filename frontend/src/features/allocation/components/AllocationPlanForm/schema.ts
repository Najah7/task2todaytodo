import { z } from "zod"

const allocationRuleSchema = z.object({
  id: z.string().optional(),
  revision: z.number().optional(),
  weekday: z.enum(["0", "1", "2", "3", "4", "5", "6"]),
  startTime: z.string().regex(/^\d{2}:\d{2}$/, "weeklyPlan.plan.invalidTime"),
  endTime: z.string().regex(/^\d{2}:\d{2}$/, "weeklyPlan.plan.invalidTime"),
  endDayOffset: z.enum(["0", "1"]),
  timezone: z.string().min(1),
  pausedReason: z.string().optional(),
})

export const allocationPlanSchema = z.object({
  projects: z.array(z.object({
    projectId: z.string().min(1),
    targetMinutes: z.number().int().min(0, "weeklyPlan.plan.invalidTarget"),
    hasTarget: z.boolean(),
    rules: z.array(allocationRuleSchema),
  })),
})

export type AllocationPlanValues = z.infer<typeof allocationPlanSchema>
