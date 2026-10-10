import type { RestProjectResponse } from "~/api/generated/projects"
import type { ProjectFormValues } from "~/features/project/components/ProjectForm/schema"

export function projects2projectFormValues(projects: RestProjectResponse[]): ProjectFormValues[] {
  return projects.map((project) => ({
    title: project.title,
    goal: project.goal,
    description: project.description,
    type: project.type.value,
    priority: project.priority.value,
    startDate: project.start_date ?? "",
    endDate: project.end_date ?? "",
  }))
}
