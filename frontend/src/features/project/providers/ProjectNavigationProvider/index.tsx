import type { ReactNode } from "react"
import { useNavigate, useSearchParams } from "react-router"
import { projectPageTokenChanges2searchParams } from "~/features/project/converters/projectPageTokenChanges2searchParams"
import { projectSearchParams2projectListStates } from "~/features/project/converters/projectSearchParams2projectListStates"
import { projectSortChanges2searchParams } from "~/features/project/converters/projectSortChanges2searchParams"
import { projectTabChanges2searchParams } from "~/features/project/converters/projectTabChanges2searchParams"
import { ProjectNavigationContext, type ProjectNavigation } from "./context"

export function ProjectNavigationProvider({ children }: { children: ReactNode }) {
  const navigate = useNavigate()
  const [searchParams, setSearchParams] = useSearchParams()
  const state = projectSearchParams2projectListStates([searchParams])[0]!
  const value: ProjectNavigation = {
    state,
    cancel: () => navigate("/projects"),
    afterSave: (status) => navigate(`/projects?status=${encodeURIComponent(status ?? "open")}`),
    create: () => navigate("/projects/new"),
    edit: (projectId) => navigate(`/projects/${encodeURIComponent(projectId)}/edit`),
    selectTab: (tab) => setSearchParams(projectTabChanges2searchParams([{ searchParams, tab }])[0]!, { preventScrollReset: true }),
    changeSort: (sortColumn, sortOrder) => setSearchParams(projectSortChanges2searchParams([{ searchParams, sortColumn, sortOrder }])[0]!, { preventScrollReset: true }),
    first: (options) => setSearchParams((current) => projectPageTokenChanges2searchParams([{ searchParams: current }])[0]!, { preventScrollReset: true, ...options }),
    previous: (pageToken) => setSearchParams(projectPageTokenChanges2searchParams([{ searchParams, pageToken }])[0]!, { preventScrollReset: true }),
    next: (pageToken) => setSearchParams(projectPageTokenChanges2searchParams([{ searchParams, pageToken }])[0]!, { preventScrollReset: true }),
  }

  return <ProjectNavigationContext.Provider value={value}>{children}</ProjectNavigationContext.Provider>
}
