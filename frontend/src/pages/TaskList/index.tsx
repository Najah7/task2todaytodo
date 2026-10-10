import { useState } from "react"
import { useMutation, useQueries, useQuery, useQueryClient } from "@tanstack/react-query"
import { useLocation, useNavigate, useSearchParams } from "react-router"
import {
  GetTasksDueFilter,
  GetTasksStatus,
  getGetTasksQueryKey,
  getGetTasksTaskIdActionItemsQueryKey,
  postTasksTaskIdActionItemsIdcomplete,
  postTasksTaskIdActionItemsIdreopen,
  useGetTasks,
  type RestActionItemResponse,
} from "~/api/generated/tasks"
import {
  getGetProjectsQueryKey,
} from "~/api/generated/projects"
import { useI18n, useLanguage } from "~/features/i18n/hooks"
import { notify } from "~/features/shared/notification"
import TaskListPage from "~/features/task/components/TaskListPage"
import { TaskListProvider } from "~/features/task/providers/TaskList"
import { tasks2taskListRowViews } from "~/features/task/converters/tasks2taskListRowViews"
import { taskListResponses2statusTabs, taskListResponses2summaryViews } from "~/features/task/converters/taskListResponses2summaryViews"
import {
  taskFiltersChanges2searchParams,
  taskListStates2apiParams,
  taskPageTokenChanges2searchParams,
  taskSearchParams2states,
  taskSortChanges2searchParams,
  taskStatusChanges2searchParams,
} from "~/features/task/converters/taskSearchParams"
import type { TaskActionItemRowView, TaskActionItemsQueryState, TaskListFiltersView, TaskSelectOption } from "~/features/task/types"
import type { TaskListState } from "~/features/task/converters/taskSearchParams"
import { getAllActiveProjects, getAllTaskActionItems } from "~/features/task/queries"

const projectOptionsKey = ["task-list", "active-project-options"] as const
const dueFilterLabels: Record<string, "tasks.filter.due.all" | "tasks.filter.due.overdue" | "tasks.filter.due.today" | "tasks.filter.due.dueSoon" | "tasks.filter.due.noDue"> = {
  all: "tasks.filter.due.all",
  overdue: "tasks.filter.due.overdue",
  today: "tasks.filter.due.today",
  due_soon: "tasks.filter.due.dueSoon",
  no_due: "tasks.filter.due.noDue",
}

export default function TaskList() {
  const i18n = useI18n()
  const { language } = useLanguage()
  const [searchParams, setSearchParams] = useSearchParams()
  const navigate = useNavigate()
  const location = useLocation()
  const [expandedIds, setExpandedIds] = useState<string[]>([])
  const [busyActionItemKey, setBusyActionItemKey] = useState<string>()
  const state = taskSearchParams2states([searchParams])[0]!
  const params = taskListStates2apiParams([state])[0]!
  const tasksQuery = useGetTasks(params, { query: { retry: false } })
  const projectsQuery = useQuery({
    queryKey: projectOptionsKey,
    queryFn: getAllActiveProjects,
    staleTime: 5 * 60 * 1000,
    retry: false,
  })
  const queryClient = useQueryClient()
  const actionItemMutation = useMutation({
    mutationFn: ({ item, completed }: { item: TaskActionItemRowView; completed: boolean }) => {
      const actionItemId = item.seriesId ?? item.actionItemId
      const request = item.seriesId && item.occurrenceDate ? { occurrence_date: item.occurrenceDate } : undefined
      return completed
        ? postTasksTaskIdActionItemsIdcomplete(item.taskId, actionItemId, request)
        : postTasksTaskIdActionItemsIdreopen(item.taskId, actionItemId, request)
    },
    onSuccess: async () => {
      notify.success(i18n("tasks.toast.actionItemUpdated"))
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: getGetTasksQueryKey() }),
        queryClient.invalidateQueries({ queryKey: getGetProjectsQueryKey() }),
        ...expandedIds.map((taskId) => queryClient.invalidateQueries({ queryKey: getGetTasksTaskIdActionItemsQueryKey(taskId) })),
      ])
    },
    onError: async () => {
      notify.error(i18n("tasks.form.genericError"))
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: getGetTasksQueryKey() }),
        queryClient.invalidateQueries({ queryKey: getGetProjectsQueryKey() }),
        ...expandedIds.map((taskId) => queryClient.invalidateQueries({ queryKey: getGetTasksTaskIdActionItemsQueryKey(taskId) })),
      ])
    },
    onSettled: () => setBusyActionItemKey(undefined),
  })
  const visibleExpandedIds = expandedIds.filter((taskId) => tasksQuery.data?.items?.some((task) => task.id === taskId))
  const actionItemQueries = useQueries({
    queries: visibleExpandedIds.map((taskId) => ({
      queryKey: [...getGetTasksTaskIdActionItemsQueryKey(taskId, { page_size: 100 }), "all-pages"],
      queryFn: () => getAllTaskActionItems(taskId),
      retry: false,
    })),
  })
  const taskItemsById = Object.fromEntries(visibleExpandedIds.map((taskId, index) => [taskId, actionItemQueries[index]?.data ?? []])) as Record<string, RestActionItemResponse[]>
  const actionItemsQueryStates = Object.fromEntries(visibleExpandedIds.map((taskId, index) => {
    const query = actionItemQueries[index]
    const status = query?.isLoading || query?.isFetching ? "loading" : query?.isError ? "error" : "success"
    return [taskId, status]
  })) as Record<string, TaskActionItemsQueryState | undefined>
  const getStatusLabel = (status: string) => i18n(taskStatusLabel(status))
  const rows = tasks2taskListRowViews((tasksQuery.data?.items ?? []).map((task) => ({
    task,
    actionItems: taskItemsById[task.id ?? ""] ?? [],
    language,
    unassignedProject: i18n("tasks.project.unassigned"),
    noDeadline: i18n("tasks.deadline.none"),
    getStatusLabel,
  })))
  const summary = taskListResponses2summaryViews([tasksQuery.data], language, i18n)[0]!
  const tabs = taskListResponses2statusTabs([tasksQuery.data], [state.status])[0]!
  const projectItems = projectsQuery.data ?? []
  const filters: TaskListFiltersView = {
    projectOptions: [
      { value: "", label: i18n("tasks.filter.allProjects") },
      ...projectItems.map((project) => ({ value: project.id, label: project.title })),
    ],
    dueOptions: Object.entries(dueFilterLabels).map(([value, label]) => ({ value, label: i18n(label) })),
    values: { projectId: state.projectId ?? "", dueFilter: state.dueFilter ?? GetTasksDueFilter.all, title: state.title },
  }
  const sortOptions: TaskSelectOption[] = [
    { value: "due_date:asc", label: i18n("tasks.sort.dueAsc") },
    { value: "due_date:desc", label: i18n("tasks.sort.dueDesc") },
    { value: "title:asc", label: i18n("tasks.sort.titleAsc") },
    { value: "created_at:desc", label: i18n("tasks.sort.createdDesc") },
  ]
  const listData = {
    tabs,
    filters,
    sort: `${state.sortBy}:${state.sortOrder}`,
    sortOptions,
    rows,
    summary,
    expandedIds,
    busyActionItemKey,
    actionItemsQueryStates,
    loading: tasksQuery.isLoading,
    error: Boolean(tasksQuery.error && !tasksQuery.data),
    canGoFirst: Boolean(state.pageToken && tasksQuery.data?.items?.length === 0),
    previousDisabled: !tasksQuery.data?.previous_page_token || tasksQuery.isLoading,
    nextDisabled: !tasksQuery.data?.next_page_token || tasksQuery.isLoading,
    optionsLoading: projectsQuery.isLoading,
    optionsError: Boolean(projectsQuery.isError),
  }
  const listNavigation = {
    selectStatus: (value: string) => setSearchParams(taskStatusChanges2searchParams([{ searchParams, status: value as typeof state.status }])[0]!, { preventScrollReset: true }),
    changeFilter,
    changeSort,
    first: () => setSearchParams(taskPageTokenChanges2searchParams([{ searchParams }])[0]!, { preventScrollReset: true }),
    previous: () => setSearchParams(taskPageTokenChanges2searchParams([{ searchParams, pageToken: tasksQuery.data?.previous_page_token }])[0]!, { preventScrollReset: true }),
    next: () => setSearchParams(taskPageTokenChanges2searchParams([{ searchParams, pageToken: tasksQuery.data?.next_page_token }])[0]!, { preventScrollReset: true }),
  }
  const listActions = {
    create: () => navigate(`/tasks/new${state.projectId ? `?project_id=${encodeURIComponent(state.projectId)}` : ""}`, { state: { from: `${location.pathname}${location.search}` } }),
    edit: (taskId: string) => navigate(`/tasks/${encodeURIComponent(taskId)}/edit`, { state: { from: `${location.pathname}${location.search}` } }),
    toggleExpanded,
    toggleActionItem,
    retryActionItems: (taskId: string) => void queryClient.invalidateQueries({ queryKey: getGetTasksTaskIdActionItemsQueryKey(taskId) }),
    retry: () => void tasksQuery.refetch(),
    retryOptions: () => { void projectsQuery.refetch() },
  }

  function toggleExpanded(taskId: string) {
    setExpandedIds((current) => current.includes(taskId) ? current.filter((id) => id !== taskId) : [...current, taskId])
  }

  function toggleActionItem(item: TaskActionItemRowView) {
    if (actionItemMutation.isPending) return
    setBusyActionItemKey(item.occurrenceKey)
    actionItemMutation.mutate({ item, completed: !item.completed })
  }

  function changeFilter(next: TaskListFiltersView["values"]) {
    const nextParams = taskFiltersChanges2searchParams([{
      searchParams,
      projectId: next.projectId,
      dueFilter: next.dueFilter as typeof state.dueFilter,
      title: next.title,
    }])[0]!
    setSearchParams(nextParams, { preventScrollReset: true, replace: next.title !== state.title })
  }

  function changeSort(sort: string) {
    const [sortBy, sortOrder] = sort.split(":") as [NonNullable<TaskListState["sortBy"]>, typeof state.sortOrder]
    const next = taskSortChanges2searchParams([{ searchParams, sortBy, sortOrder }])[0]!
    setSearchParams(next, { preventScrollReset: true })
  }

  return (
    <>
      <TaskListProvider data={listData} navigation={listNavigation} actions={listActions}>
        <TaskListPage />
      </TaskListProvider>
    </>
  )
}

function taskStatusLabel(status: string): "tasks.status.open" | "tasks.status.inProgress" | "tasks.status.pending" | "tasks.status.waitingOnOthers" | "tasks.status.done" {
  if (status === GetTasksStatus.in_progress) return "tasks.status.inProgress"
  if (status === GetTasksStatus.pending) return "tasks.status.pending"
  if (status === GetTasksStatus.waiting_on_others) return "tasks.status.waitingOnOthers"
  if (status === GetTasksStatus.done) return "tasks.status.done"
  return "tasks.status.open"
}
