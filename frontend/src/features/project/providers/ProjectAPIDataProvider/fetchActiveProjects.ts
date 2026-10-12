import { GetProjectsView, getProjects } from "~/api/generated/projects"
import type { GetProjectsParams, RestProjectResponse } from "~/api/generated/projects"

export async function fetchActiveProjects(signal?: AbortSignal): Promise<RestProjectResponse[]> {
  const projects: RestProjectResponse[] = []
  const seenTokens = new Set<string>()
  let pageToken: string | undefined
  do {
    const params: GetProjectsParams = {
      view: GetProjectsView.active,
      page_size: 100,
      ...(pageToken ? { page_token: pageToken } : {}),
    }
    const response = await getProjects(params, { signal })
    projects.push(...response.items)
    pageToken = response.next_page_token || undefined
    if (pageToken && seenTokens.has(pageToken)) throw new Error("Project pagination token repeated")
    if (pageToken) seenTokens.add(pageToken)
  } while (pageToken)
  return projects
}
