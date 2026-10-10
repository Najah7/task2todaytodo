import { getProjects, GetProjectsView } from "~/api/generated/projects"
import type { RestProjectResponse } from "~/api/generated/projects"
import { getTasksTaskIdActionItems } from "~/api/generated/tasks"
import type { RestActionItemResponse } from "~/api/generated/tasks"

export async function getAllActiveProjects(): Promise<RestProjectResponse[]> {
  const projects: RestProjectResponse[] = []
  let pageToken: string | undefined
  do {
    const response = await getProjects({ view: GetProjectsView.active, page_size: 100, ...(pageToken ? { page_token: pageToken } : {}) })
    projects.push(...response.items)
    pageToken = response.next_page_token || undefined
  } while (pageToken)
  return projects
}

export async function getAllTaskActionItems(taskId: string): Promise<RestActionItemResponse[]> {
  const items: RestActionItemResponse[] = []
  const seenTokens = new Set<string>()
  let pageToken: string | undefined
  do {
    const response = await getTasksTaskIdActionItems(taskId, { page_size: 100, ...(pageToken ? { page_token: pageToken } : {}) })
    items.push(...response.items ?? [])
    pageToken = response.next_page_token || undefined
    if (pageToken && seenTokens.has(pageToken)) throw new Error("ActionItem pagination token repeated")
    if (pageToken) seenTokens.add(pageToken)
  } while (pageToken)
  return items
}
