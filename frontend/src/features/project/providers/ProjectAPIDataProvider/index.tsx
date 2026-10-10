import { useEffect, type ReactNode } from "react"
import { useSearchParams } from "react-router"
import { useGetProjects, useGetProjectsId, useGetProjectsOptions } from "~/api/generated/projects"
import { projectListStates2apiParams } from "~/features/project/converters/projectListStates2apiParams"
import { projectSearchParams2projectListStates } from "~/features/project/converters/projectSearchParams2projectListStates"
import { getProjectErrorMessageKey } from "~/features/project/errors"
import { useI18n } from "~/features/i18n/hooks"
import { notify } from "~/features/shared/notification"
import { ProjectAPIDataContext, type ProjectAPIData } from "./context"

export function ProjectAPIDataProvider({
  projectId,
  list = false,
  includeOptions,
  children,
}: {
  projectId?: string
  list?: boolean
  includeOptions?: boolean
  children: ReactNode
}) {
  const i18n = useI18n()
  const [searchParams] = useSearchParams()
  const listState = projectSearchParams2projectListStates([searchParams])[0]!
  const listParams = projectListStates2apiParams([listState])[0]!
  const projectsQuery = useGetProjects(listParams, { query: { enabled: list, retry: false } })
  const projectQuery = useGetProjectsId(projectId ?? "", { query: { enabled: projectId !== undefined, retry: false } })
  const shouldFetchOptions = includeOptions ?? !list
  const optionsQuery = useGetProjectsOptions({ query: { enabled: shouldFetchOptions && (list ? projectsQuery.isSuccess : projectId === undefined || projectQuery.isSuccess), retry: false } })
  const project = projectQuery.data
  const projectErrorKey = projectQuery.error ? getProjectErrorMessageKey(projectQuery.error) : undefined
  const projectsErrorKey = projectsQuery.error ? getProjectErrorMessageKey(projectsQuery.error) : undefined
  const optionsErrorKey = optionsQuery.error && !projectQuery.error ? getProjectErrorMessageKey(optionsQuery.error) : undefined

  useEffect(() => {
    if (!list && projectErrorKey) notify.error(i18n(projectErrorKey))
    else if (!list && optionsErrorKey) notify.error(i18n(optionsErrorKey))
  }, [i18n, list, optionsErrorKey, projectErrorKey])

  const formState: ProjectAPIData["state"] = projectId === undefined
    ? optionsQuery.data
      ? "ready"
      : optionsQuery.isLoading
        ? "loading"
        : optionsErrorKey === "projects.error.forbidden" ? "forbidden" : "error"
    : !project
      ? projectQuery.isLoading
        ? "loading"
        : projectErrorKey === "projects.error.forbidden" ? "forbidden" : "error"
      : !project.can_update || project.deleted_at !== null
        ? "forbidden"
        : !optionsQuery.data
          ? optionsQuery.isLoading
            ? "loading"
            : optionsErrorKey === "projects.error.forbidden" ? "forbidden" : "error"
          : "ready"

  const state: ProjectAPIData["state"] = list
    ? projectsQuery.data && (!shouldFetchOptions || optionsQuery.data)
      ? "ready"
      : projectsQuery.isLoading
        ? "loading"
        : projectsErrorKey === "projects.error.forbidden"
          ? "forbidden"
          : projectsQuery.isError || optionsQuery.isError
            ? "error"
            : "loading"
    : formState

  const value: ProjectAPIData = {
    state,
    list,
    ...(projectId === undefined ? {} : { projectId }),
    project,
    options: optionsQuery.data,
    projectsQuery,
    optionsQuery,
  }

  return <ProjectAPIDataContext.Provider value={value}>{children}</ProjectAPIDataContext.Provider>
}
