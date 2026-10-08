import { z } from "zod"

export const projectFormSchema = z.object({
  title: z.string().trim().min(1, "projects.form.requiredTitle"),
  goal: z.string(),
  description: z.string(),
  type: z.string().min(1, "projects.form.requiredType"),
  priority: z.string().min(1, "projects.form.requiredPriority"),
  startDate: z.string(),
  endDate: z.string(),
}).superRefine(({ startDate, endDate }, context) => {
  if (startDate && endDate && endDate < startDate) {
    context.addIssue({
      code: "custom",
      path: ["endDate"],
      message: "projects.form.dateOrder",
    })
  }
})

export type ProjectFormValues = z.infer<typeof projectFormSchema>

export const emptyProjectFormValues: ProjectFormValues = {
  title: "",
  goal: "",
  description: "",
  type: "other",
  priority: "low",
  startDate: "",
  endDate: "",
}
