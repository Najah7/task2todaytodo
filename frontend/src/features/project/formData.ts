import type {
  RestProjectCreateRequest,
  RestProjectResponse,
  RestProjectUpdateRequestSchema,
} from "~/api/generated/projects"
import type { Language } from "~/features/i18n/messages"
import type { ProjectFormValues } from "~/features/project/components/ProjectForm/schema"

export function toProjectFormValues(project: RestProjectResponse): ProjectFormValues {
  return {
    title: project.title,
    goal: project.goal,
    description: project.description,
    type: project.type.value,
    priority: project.priority.value,
    startDate: project.start_date ?? "",
    endDate: project.end_date ?? "",
  }
}

export function toProjectCreateRequest(values: ProjectFormValues): RestProjectCreateRequest {
  return {
    title: values.title.trim(),
    goal: values.goal,
    description: values.description,
    type: values.type,
    priority: values.priority,
    start_date: values.startDate || null,
    end_date: values.endDate || null,
  }
}

export function toProjectUpdateRequest(values: ProjectFormValues): RestProjectUpdateRequestSchema {
  return {
    title: values.title.trim(),
    goal: values.goal || null,
    description: values.description || null,
    type: values.type,
    priority: values.priority,
    start_date: values.startDate || null,
    end_date: values.endDate || null,
  }
}

export function projectOptionLabel(option: { label: string; label_jp: string }, language: Language) {
  return language === "ja" ? option.label_jp : option.label
}
