import { z } from "zod"

export const allocationEditorSchema = z.object({
  projectId: z.string().min(1, "weeklyPlan.editor.projectRequired"),
  startAt: z.string().min(1, "weeklyPlan.plan.required"),
  endAt: z.string().min(1, "weeklyPlan.plan.required"),
  repeatWeekly: z.boolean(),
}).refine((value) => value.endAt > value.startAt, {
  path: ["endAt"],
  message: "weeklyPlan.editor.timeOrder",
})

export type AllocationEditorValues = z.infer<typeof allocationEditorSchema>
