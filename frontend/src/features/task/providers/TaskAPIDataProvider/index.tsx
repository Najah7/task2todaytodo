import { useEffect, type ReactNode } from "react"
import { useQuery } from "@tanstack/react-query"
import { getGetTasksTaskIdActionItemsQueryKey, getGetTasksIdQueryOptions, useGetTasks } from "~/api/generated/tasks"
import { useGetProjectsOptions } from "~/api/generated/projects"
import { useSearchParams } from "react-router"
import { getAllActiveProjects, getAllTaskActionItems } from "~/features/task/queries"
import { taskListStates2apiParams, taskSearchParams2states } from "~/features/task/converters/taskSearchParams"
import { TaskAPIDataContext, type TaskAPIDataLoadState } from "./context"
import { ApiError } from "~/api/http"
import { useI18n } from "~/features/i18n/hooks"
import { notify } from "~/features/shared/notification"
import { taskErrorMessageKey } from "~/features/task/errors"

const activeProjectsQueryKey = ["tasks", "active-projects"] as const

type Props = {
  children: ReactNode
  list?: boolean
  taskId?: string
  includeOptions?: boolean
}

export function TaskAPIDataProvider({ children, list = false, taskId, includeOptions }: Props) {
  const i18n = useI18n()
  const [searchParams] = useSearchParams()
  const shouldFetchOptions = includeOptions ?? !list
  const listState = taskSearchParams2states([searchParams])[0]!
  const listParams = taskListStates2apiParams([listState])[0]!
  const tasksQuery = useGetTasks(listParams, { query: { enabled: list, retry: false } })
  const taskQuery = useQuery({ ...getGetTasksIdQueryOptions(taskId ?? ""), enabled: Boolean(taskId), retry: false })
  const task = taskQuery.data?.task
  const actionItemsQuery = useQuery({
    queryKey: [...getGetTasksTaskIdActionItemsQueryKey(taskId ?? "", { page_size: 100 }), "all-pages"],
    queryFn: () => getAllTaskActionItems(taskId ?? ""),
    enabled: Boolean(task),
    retry: false,
  })
  const projectsQuery = useQuery({ queryKey: activeProjectsQueryKey, queryFn: getAllActiveProjects, staleTime: 5 * 60 * 1000, retry: false })
  const optionsQuery = useGetProjectsOptions({ query: { enabled: shouldFetchOptions && (!taskId || Boolean(task)), retry: false } })

  const requiredQueries = list
    ? [tasksQuery, projectsQuery, ...(shouldFetchOptions ? [optionsQuery] : [])]
    : taskId
      ? [taskQuery, actionItemsQuery, projectsQuery, ...(shouldFetchOptions ? [optionsQuery] : [])]
      : [projectsQuery, ...(shouldFetchOptions ? [optionsQuery] : [])]
  const errorQuery = requiredQueries.find((query) => query.isError && query.data === undefined)
  const missingTask = Boolean(taskId && taskQuery.isSuccess && !task)
  const forbidden = Boolean(
    (taskId && task && !task.can_update) ||
    requiredQueries.some((query) => query.data === undefined && query.isError && isForbiddenError(query.error)),
  )
  const ready = !forbidden && !missingTask && requiredQueries.every((query) => query.data !== undefined) && (!taskId || Boolean(task))
  const state: TaskAPIDataLoadState = forbidden ? "forbidden" : ready ? "ready" : errorQuery || missingTask ? "error" : "loading"

  const loadError = taskId
    ? taskQuery.error ?? actionItemsQuery.error ?? projectsQuery.error ?? optionsQuery.error
    : projectsQuery.error ?? optionsQuery.error
  useEffect(() => {
    if (!list && loadError) notify.error(i18n(taskErrorMessageKey(loadError)))
  }, [i18n, list, loadError])

  return (
    <TaskAPIDataContext.Provider value={{
      state,
      list,
      taskId,
      task,
      tasksQuery,
      taskQuery,
      actionItemsQuery,
      projectsQuery,
      optionsQuery,
    }}>
      {children}
    </TaskAPIDataContext.Provider>
  )
}

function isForbiddenError(error: unknown): boolean {
  return error instanceof ApiError && (error.status === 401 || error.status === 403)
}
