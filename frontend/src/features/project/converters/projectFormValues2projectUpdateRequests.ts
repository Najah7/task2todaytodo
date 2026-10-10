import type { RestProjectUpdateRequestSchema } from "~/api/generated/projects"
import type { ProjectFormValues } from "~/features/project/components/ProjectForm/schema"

export function projectFormValues2projectUpdateRequests(values: ProjectFormValues[]): RestProjectUpdateRequestSchema[] {
  return values.map((value) => ({
    title: value.title.trim(),
    goal: value.goal || null,
    description: value.description || null,
    type: value.type,
    priority: value.priority,
    start_date: value.startDate || null,
    end_date: value.endDate || null,
  }))
}
