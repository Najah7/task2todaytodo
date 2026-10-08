import type { GetProjectsStatus, RestProjectListResponse, RestProjectResponse } from "~/api/generated/projects"

export function removeStatusChangedProject(
  previous: RestProjectListResponse,
  project: RestProjectResponse,
  nextStatus: GetProjectsStatus,
): RestProjectListResponse {
  const statusCounts = { ...previous.summary.status_counts }
  statusCounts[project.status] = Math.max(0, statusCounts[project.status] - 1)
  statusCounts[nextStatus] += 1
  return {
    ...previous,
    items: previous.items.filter((item) => item.id !== project.id),
    summary: {
      ...previous.summary,
      total_count: Math.max(0, previous.summary.total_count - 1),
      status_counts: statusCounts,
    },
  }
}
