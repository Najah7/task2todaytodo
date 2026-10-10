import { useContext } from "react"
import type { GetProjectsStatus as ProjectStatus, RestProjectResponse, RestProjectTaskStatusResponse } from "~/api/generated/projects"
import { useI18n, useLanguage } from "~/features/i18n/hooks"
import { ProjectAPIDataContext } from "~/features/project/providers/ProjectAPIDataProvider/context"
import { ProjectActionContext } from "~/features/project/providers/ProjectActionProvider/context"
import { ProjectNavigationContext } from "~/features/project/providers/ProjectNavigationProvider/context"
import { projectTabLabels } from "~/features/project/components/ProjectStatusTab/constants"
import SeachSelect from "~/features/shared/components/SeachSelect"
import styles from "./index.module.css"

type Props = {
  project: RestProjectResponse
}

export default function ProjectStatusControl({ project }: Props) {
  const i18n = useI18n()
  const { language } = useLanguage()
  const { optionsQuery } = useContext(ProjectAPIDataContext)!
  const { state } = useContext(ProjectNavigationContext)!
  const { busy, changeStatus } = useContext(ProjectActionContext)!
  const statusOptions = optionsQuery?.data?.statuses ?? []
  if (state.tab === "trash") {
    return projectRowLabel(statusOptions.find((status) => status.value === project.status), language, i18n(projectTabLabels[project.status]))
  }
  return (
    <SeachSelect
      ariaLabel={i18n("projects.status.select", { title: project.title })}
      className={styles.statusSelect}
      disabled={!project.can_update || busy || statusOptions.length === 0}
      value={project.status}
      options={statusOptions.flatMap((option) => option.value ? [{
        value: option.value,
        label: projectRowLabel(option, language, i18n(projectTabLabels[option.value as ProjectStatus] ?? "projects.status.open")),
      }] : [])}
      onChange={(value) => changeStatus(project, value)}
    />
  )
}

function projectRowLabel(option: RestProjectTaskStatusResponse | undefined, language: string, fallback: string) {
  if (!option) return fallback
  return language === "ja" ? option.label_jp || option.label || fallback : option.label || option.label_jp || fallback
}
