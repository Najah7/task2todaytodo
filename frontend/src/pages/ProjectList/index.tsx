import { useEffect, useState } from "react"
import { useMutation, useQueryClient } from "@tanstack/react-query"
import { useNavigate, useSearchParams } from "react-router"
import {
  deleteProjectsId,
  getGetProjectsIdQueryKey,
  getGetProjectsQueryKey,
  GetProjectsSortOrder,
  GetProjectsStatus,
  GetProjectsView,
  patchProjectsIdStatus,
  postProjectsIdRestore,
  useGetProjects,
  useGetProjectsOptions,
} from "~/api/generated/projects"
import type {
  GetProjectsSortOrder as ProjectSortOrder,
  GetProjectsStatus as ProjectStatus,
  RestProjectListResponse,
  RestProjectResponse,
} from "~/api/generated/projects"
import { useI18n } from "~/features/i18n/hooks"
import ConfirmationDialog from "~/features/project/components/ConfirmationDialog"
import ProjectCreateButton from "~/features/project/components/ProjectCreateButton"
import ProjectPaginationFooter from "~/features/project/components/ProjectPaginationFooter"
import ProjectStats from "~/features/project/components/ProjectStats"
import ProjectStatusTab from "~/features/project/components/ProjectStatusTab"
import ProjectTable from "~/features/project/components/ProjectTable"
import { ProjectListProvider } from "~/features/project/providers/ProjectList"
import { getProjectErrorMessageKey } from "~/features/project/errors"
import { projectListStates2apiParams } from "~/features/project/converters/projectListStates2apiParams"
import { projectPageTokenChanges2searchParams } from "~/features/project/converters/projectPageTokenChanges2searchParams"
import { projectSortChanges2searchParams } from "~/features/project/converters/projectSortChanges2searchParams"
import { projectTabChanges2searchParams } from "~/features/project/converters/projectTabChanges2searchParams"
import { projectTabLabels } from "~/features/project/constants"
import PageHeading from "~/features/shared/components/PageHeading"
import type { ProjectListState, ProjectSortColumn, ProjectTab } from "~/features/project/types"
import { notify } from "~/features/shared/notification"
import controls from "~/styles/controls.module.css"
import styles from "./index.module.css"

const projectsQueryPrefix = getGetProjectsQueryKey()
type StatusVariables = { project: RestProjectResponse; status: ProjectStatus }
type RevisionVariables = { project: RestProjectResponse }

function Loading() {
  const i18n = useI18n()

  return <p className={`${styles.message} text-body`} role="status">{i18n("projects.loading")}</p>
}

function Alert({ onRetry }: { onRetry: () => void }) {
  const i18n = useI18n()

  return (
    <div className={styles.message} role="alert">
      <p className="text-body">{i18n("projects.loadError")}</p>
      <button className={`${controls.button} ${controls.outlineButton} ${controls.focusRing} text-button-small`} type="button" onClick={onRetry}>{i18n("projects.retry")}</button>
    </div>
  )
}

function Empty() {
  const i18n = useI18n()

  return <p className={`${styles.message} text-body`}>{i18n("projects.empty")}</p>
}

function ProjectTableView({ query }: { query: ReturnType<typeof useGetProjects<RestProjectListResponse>> }) {
  return query.isLoading ? (
    <Loading />
  ) : query.error && !query.data ? (
    <Alert onRetry={() => query.refetch()} />
  ) : !query.data?.items.length ? (
    <Empty />
  ) : (
    <ProjectTable />
  )
}

function isProjectStatus(value: string): value is ProjectStatus {
  return Object.values(GetProjectsStatus).includes(value as ProjectStatus)
}

function readProjectListState(search: URLSearchParams): ProjectListState {
  const status = search.get("status")
  const view = search.get("view")
  const sortColumn = search.get("sort_by")
  const sortOrder = search.get("sort_order")
  const tab: ProjectTab = view === GetProjectsView.trash
    ? "trash"
    : status && isProjectStatus(status)
      ? status
      : GetProjectsStatus.in_progress
  const validSortColumn: ProjectSortColumn = sortColumn === "title" || sortColumn === "progress" || sortColumn === "end_date" || sortColumn === "remaining_days"
    ? sortColumn
    : "end_date"

  return {
    tab,
    sortColumn: validSortColumn,
    sortOrder: sortOrder === GetProjectsSortOrder.desc ? GetProjectsSortOrder.desc : GetProjectsSortOrder.asc,
    pageToken: search.get("page_token") || undefined,
  }
}

function removeStatusChangedProject(
  previous: RestProjectListResponse,
  project: RestProjectResponse,
  nextStatus: ProjectStatus,
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

export default function ProjectList() {
  const i18n = useI18n()
  const navigate = useNavigate()
  const [searchParams, setSearchParams] = useSearchParams()
  const [projectToTrash, setProjectToTrash] = useState<RestProjectResponse | null>(null)
  const state = readProjectListState(searchParams)
  const params = projectListStates2apiParams([state])[0]!
  const query = useGetProjects(params, { query: { retry: false } })
  const optionsQuery = useGetProjectsOptions({ query: { enabled: query.isSuccess, retry: false } })
  const queryClient = useQueryClient()
  const queryKey = getGetProjectsQueryKey(params)
  const listErrorKey = query.error ? getProjectErrorMessageKey(query.error) : undefined
  const optionsErrorKey = optionsQuery.error && query.isSuccess ? getProjectErrorMessageKey(optionsQuery.error) : undefined
  const statusOptions = optionsQuery.data?.statuses ?? []

  useEffect(() => {
    if (listErrorKey) notify.error(i18n(listErrorKey))
  }, [i18n, listErrorKey])
  useEffect(() => {
    if (optionsErrorKey) notify.error(i18n(optionsErrorKey))
  }, [i18n, optionsErrorKey])
  useEffect(() => {
    if (!state.pageToken || query.isFetching || !query.data || query.data.items.length > 0 || query.data.previous_page_token) return
    setSearchParams((current) => projectPageTokenChanges2searchParams([{ searchParams: current }])[0]!, { preventScrollReset: true, replace: true })
  }, [state.pageToken, query.isFetching, query.data, setSearchParams])

  const statusMutation = useMutation({
    mutationFn: ({ project, status }: StatusVariables) => patchProjectsIdStatus(
      project.id, { status }, { headers: { "If-Match": `"${project.revision}"` } },
    ),
    onMutate: async ({ project, status }) => {
      await queryClient.cancelQueries({ queryKey })
      const previous = queryClient.getQueryData<RestProjectListResponse>(queryKey)
      if (previous) queryClient.setQueryData(queryKey, removeStatusChangedProject(previous, project, status))
      return { queryKey, previous }
    },
    onError: (error, _variables, context) => {
      if (context?.previous) queryClient.setQueryData(context.queryKey, context.previous)
      notify.error(i18n(getProjectErrorMessageKey(error)))
    },
    onSuccess: (project) => {
      queryClient.setQueryData(getGetProjectsIdQueryKey(project.id), project)
      notify.success(i18n("projects.toast.statusChanged"))
    },
    onSettled: () => queryClient.invalidateQueries({ queryKey: projectsQueryPrefix }),
  })
  const trashMutation = useMutation({
    mutationFn: ({ project }: RevisionVariables) => deleteProjectsId(project.id, { headers: { "If-Match": `"${project.revision}"` } }),
    onError: (error) => notify.error(i18n(getProjectErrorMessageKey(error))),
    onSuccess: () => notify.success(i18n("projects.toast.trashed")),
    onSettled: (_data, _error, { project }) => {
      setProjectToTrash(null)
      return Promise.all([
        queryClient.invalidateQueries({ queryKey: getGetProjectsIdQueryKey(project.id) }),
        queryClient.invalidateQueries({ queryKey: projectsQueryPrefix }),
      ])
    },
  })
  const restoreMutation = useMutation({
    mutationFn: ({ project }: RevisionVariables) => postProjectsIdRestore(project.id, { headers: { "If-Match": `"${project.revision}"` } }),
    onError: (error) => notify.error(i18n(getProjectErrorMessageKey(error))),
    onSuccess: (project) => {
      queryClient.setQueryData(getGetProjectsIdQueryKey(project.id), project)
      notify.success(i18n("projects.toast.restored"))
      setSearchParams(projectTabChanges2searchParams([{ searchParams, tab: project.status }])[0]!, { preventScrollReset: true })
    },
    onSettled: () => queryClient.invalidateQueries({ queryKey: projectsQueryPrefix }),
  })
  const busy = statusMutation.isPending || trashMutation.isPending || restoreMutation.isPending

  function selectTab(tab: ProjectTab) {
    setSearchParams(projectTabChanges2searchParams([{ searchParams, tab }])[0]!, { preventScrollReset: true })
  }
  function changeStatus(project: RestProjectResponse, value: string) {
    if (!isProjectStatus(value) || value === project.status || busy) return
    statusMutation.mutate({ project, status: value })
  }
  function handleRowClick(_project: RestProjectResponse) {
    // TODO Navigate to the Tasks page filtered by this Project when that page supports project filters.
  }
  function changeSort(column: ProjectSortColumn, order: ProjectSortOrder) {
    setSearchParams(projectSortChanges2searchParams([{ searchParams, sortColumn: column, sortOrder: order }])[0]!, { preventScrollReset: true })
  }
  const projectListData = {
    projects: query.data?.items ?? [],
    summary: query.data?.summary,
    statusOptions,
  }
  const projectListNavigation = {
    state,
    showFirstPage: Boolean(state.pageToken && query.data?.items.length === 0 && !query.data.previous_page_token),
    previousDisabled: !query.data?.previous_page_token || busy || query.isLoading,
    nextDisabled: !query.data?.next_page_token || busy || query.isLoading,
    selectTab,
    changeSort,
    onFirstPage: () => setSearchParams((current) => projectPageTokenChanges2searchParams([{ searchParams: current }])[0]!, { preventScrollReset: true }),
    onPrevious: () => setSearchParams(projectPageTokenChanges2searchParams([{ searchParams, pageToken: query.data?.previous_page_token }])[0]!, { preventScrollReset: true }),
    onNext: () => setSearchParams(projectPageTokenChanges2searchParams([{ searchParams, pageToken: query.data?.next_page_token }])[0]!, { preventScrollReset: true }),
  }
  const projectListActions = {
    busy,
    changeStatus,
    edit: (project: RestProjectResponse) => navigate(`/projects/${encodeURIComponent(project.id)}/edit`),
    moveToTrash: setProjectToTrash,
    restore: (project: RestProjectResponse) => restoreMutation.mutate({ project }),
    rowClick: handleRowClick,
  }

  return (
    <ProjectListProvider data={projectListData} navigation={projectListNavigation} actions={projectListActions}>
    <section className={styles.page} aria-labelledby="projects-heading">
      <header className={styles.header}>
        <PageHeading id="projects-heading" messageKey="page.projects.title" descriptionKey="projects.description" />
        <ProjectCreateButton label={i18n("projects.create")} onClick={() => navigate("/projects/new")} />
      </header>
      <ProjectStatusTab />
      <section className={`${controls.card} ${styles.listCard}`} aria-label={i18n(projectTabLabels[state.tab])}>
        <ProjectStats />
        <ProjectTableView query={query} />
        <ProjectPaginationFooter />
      </section>
      <ConfirmationDialog
        open={projectToTrash !== null} title={i18n("projects.dialog.trashTitle")} description={i18n("projects.dialog.trashDescription")}
        confirmLabel={i18n("projects.moveToTrash", { title: projectToTrash?.title ?? "" })} cancelLabel={i18n("projects.dialog.keepEditing")}
        busy={trashMutation.isPending} onConfirm={() => projectToTrash && trashMutation.mutate({ project: projectToTrash })}
        onCancel={() => setProjectToTrash(null)}
      />
    </section>
    </ProjectListProvider>
  )
}
