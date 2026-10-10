import { GetProjectsStatus } from "~/api/generated/projects"
import type { MessageKey } from "~/features/i18n/types"
import type { ProjectTab } from "./types"

export const projectTabLabels: Record<ProjectTab, MessageKey> = {
  [GetProjectsStatus.in_progress]: "projects.status.inProgress",
  [GetProjectsStatus.pending]: "projects.status.pending",
  [GetProjectsStatus.done]: "projects.status.done",
  [GetProjectsStatus.open]: "projects.status.open",
  [GetProjectsStatus.waiting_on_others]: "projects.status.waitingOnOthers",
  trash: "projects.status.trash",
}
