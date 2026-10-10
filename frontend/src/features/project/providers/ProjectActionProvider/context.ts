import { createContext } from "react"
import type { MessageKey } from "~/features/i18n/messages/types"
import type { ProjectFormValues } from "~/features/project/components/ProjectForm/schema"
import type { RestProjectResponse } from "~/api/generated/projects"

export type ProjectSubmitResult =
  | { saved: true; revision: number }
  | { saved: false; fieldErrors?: Partial<Record<keyof ProjectFormValues, MessageKey>>; conflict?: boolean }

export type ProjectActions = {
  busy: boolean
  trashBusy: boolean
  submit: (values: ProjectFormValues) => Promise<ProjectSubmitResult>
  saveComplete: (revision: number) => void
  retry: () => Promise<void>
  reloadLatest?: () => Promise<RestProjectResponse>
  changeStatus: (project: RestProjectResponse, value: string) => void
  trash: (project: RestProjectResponse) => Promise<void>
  restore: (project: RestProjectResponse) => Promise<RestProjectResponse | undefined>
}

export const ProjectActionContext = createContext<ProjectActions | null>(null)
