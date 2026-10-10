import { createContext } from "react"
import type { UseQueryResult } from "@tanstack/react-query"
import type { RestProjectOptionsResponse, RestProjectResponse } from "~/api/generated/projects"
import type { RestActionItemResponse, RestTaskDetailsResponse, RestTaskListResponse } from "~/api/generated/tasks"

export type TaskAPIDataLoadState = "loading" | "error" | "forbidden" | "ready"

export type TaskAPIData = {
  state: TaskAPIDataLoadState
  list: boolean
  taskId?: string
  task?: RestTaskDetailsResponse["task"]
  tasksQuery: UseQueryResult<RestTaskListResponse>
  taskQuery: UseQueryResult<RestTaskDetailsResponse>
  actionItemsQuery: UseQueryResult<RestActionItemResponse[]>
  projectsQuery: UseQueryResult<RestProjectResponse[]>
  optionsQuery: UseQueryResult<RestProjectOptionsResponse>
}

export const TaskAPIDataContext = createContext<TaskAPIData | null>(null)
