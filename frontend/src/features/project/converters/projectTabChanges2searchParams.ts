import { GetProjectsView } from "~/api/generated/projects"
import type { ProjectTab } from "~/features/project/providers/ProjectNavigationProvider/context"

type ProjectTabChange = { searchParams: URLSearchParams; tab: ProjectTab }

export function projectTabChanges2searchParams(changes: ProjectTabChange[]): URLSearchParams[] {
  return changes.map(({ searchParams, tab }) => {
    const next = new URLSearchParams(searchParams)
    next.delete("page_token")
    if (tab === "trash") {
      next.delete("status")
      next.set("view", GetProjectsView.trash)
    } else {
      next.delete("view")
      next.set("status", tab)
    }
    return next
  })
}
