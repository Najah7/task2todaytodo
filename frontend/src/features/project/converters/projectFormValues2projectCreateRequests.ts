import type { RestProjectCreateRequest } from "~/api/generated/projects"
import type { ProjectFormValues } from "~/features/project/components/ProjectForm/schema"

export function projectFormValues2projectCreateRequests(values: ProjectFormValues[]): RestProjectCreateRequest[] {
  return values.map((value) => ({
    title: value.title.trim(),
    goal: value.goal,
    description: value.description,
    type: value.type,
    priority: value.priority,
    start_date: value.startDate || null,
    end_date: value.endDate || null,
  }))
}
