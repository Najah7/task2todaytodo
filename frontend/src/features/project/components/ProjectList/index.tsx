import { useEffect, useState } from "react"
import { useMutation, useQueryClient } from "@tanstack/react-query"
import { useNavigate, useSearchParams } from "react-router"
import {
  deleteProjectsId,
  getGetProjectsIdQueryKey,
  getGetProjectsQueryKey,
  GetProjectsStatus,
  patchProjectsIdStatus,
  postProjectsIdRestore,
  useGetProjects,
  useGetProjectsOptions,
} from "~/api/generated/projects"
import type {
  GetProjectsSortOrder,
  GetProjectsStatus as ProjectStatus,
  RestProjectListResponse,
  RestProjectResponse,
} from "~/api/generated/projects"
import { useI18n, useLanguage } from "~/features/i18n/hooks"
import { messages } from "~/features/i18n/messages"
import ConfirmationDialog from "~/features/project/components/ConfirmationDialog"
import ProjectTable from "~/features/project/components/ProjectList/parts/ProjectTable"
import { getProjectErrorMessageKey } from "~/features/project/errors"
import { removeStatusChangedProject } from "~/features/project/listCache"
import {
  getProjectListParams,
  projectTabLabels,
  projectTabs,
  readProjectListState,
  setProjectPage,
  setProjectSort,
  setProjectTab,
  type ProjectSortColumn,
  type ProjectTab,
} from "~/features/project/listState"
import { notify } from "~/features/shared/notification"
import controls from "~/styles/controls.module.css"
import styles from "./index.module.css"

const projectsQueryPrefix = getGetProjectsQueryKey()

type StatusVariables = { project: RestProjectResponse; status: ProjectStatus }
type RevisionVariables = { project: RestProjectResponse }

function isProjectStatus(value: string): value is ProjectStatus {
  return Object.values(GetProjectsStatus).includes(value as ProjectStatus)
}

export default function ProjectList() {
  const i18n = useI18n()
  const { language } = useLanguage()
  const navigate = useNavigate()
  const [searchParams, setSearchParams] = useSearchParams()
  const [projectToTrash, setProjectToTrash] = useState<RestProjectResponse | null>(null)
  const state = readProjectListState(searchParams)
  const params = getProjectListParams(state)
  const query = useGetProjects(params, { query: { retry: false } })
  const optionsQuery = useGetProjectsOptions({ query: { enabled: query.isSuccess, retry: false } })
  const queryClient = useQueryClient()
  const queryKey = getGetProjectsQueryKey(params)
  const listErrorKey = query.error ? getProjectErrorMessageKey(query.error) : undefined
  const optionsErrorKey = optionsQuery.error && query.isSuccess ? getProjectErrorMessageKey(optionsQuery.error) : undefined
  const statusOptions = optionsQuery.data?.statuses ?? []

  useEffect(() => {
    if (listErrorKey) notify.error(messages[language][listErrorKey])
  }, [listErrorKey, language])

  useEffect(() => {
    if (optionsErrorKey) notify.error(messages[language][optionsErrorKey])
  }, [optionsErrorKey, language])

  useEffect(() => {
    if (!state.pageToken || query.isFetching || !query.data || query.data.items.length > 0 || query.data.previous_page_token) return
    setSearchParams((current) => setProjectPage(current), { preventScrollReset: true, replace: true })
  }, [state.pageToken, query.isFetching, query.data, setSearchParams])

  const statusMutation = useMutation({
    mutationFn: ({ project, status }: StatusVariables) => patchProjectsIdStatus(
      project.id,
      { status },
      { headers: { "If-Match": `"${project.revision}"` } },
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
    mutationFn: ({ project }: RevisionVariables) => deleteProjectsId(
      project.id,
      { headers: { "If-Match": `"${project.revision}"` } },
    ),
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
    mutationFn: ({ project }: RevisionVariables) => postProjectsIdRestore(
      project.id,
      { headers: { "If-Match": `"${project.revision}"` } },
    ),
    onError: (error) => notify.error(i18n(getProjectErrorMessageKey(error))),
    onSuccess: (project) => {
      queryClient.setQueryData(getGetProjectsIdQueryKey(project.id), project)
      notify.success(i18n("projects.toast.restored"))
      setSearchParams(setProjectTab(searchParams, project.status), { preventScrollReset: true })
    },
    onSettled: () => queryClient.invalidateQueries({ queryKey: projectsQueryPrefix }),
  })

  const busy = statusMutation.isPending || trashMutation.isPending || restoreMutation.isPending

  function selectTab(tab: ProjectTab) {
    setSearchParams(setProjectTab(searchParams, tab), { preventScrollReset: true })
  }

  function changeStatus(project: RestProjectResponse, value: string) {
    if (!isProjectStatus(value) || value === project.status || busy) return
    statusMutation.mutate({ project, status: value })
  }

  function handleRowClick(_project: RestProjectResponse) {
    // TODO Navigate to the Tasks page filtered by this Project when that page supports project filters.
  }

  function changeSort(column: ProjectSortColumn, order: GetProjectsSortOrder) {
    setSearchParams(setProjectSort(searchParams, column, order), { preventScrollReset: true })
  }

  const summary = query.data?.summary
  const summaryValues = [
    { label: i18n(projectTabLabels[state.tab]), value: summary?.total_count },
    { label: i18n("projects.summary.dueSoon"), value: summary?.due_soon_count },
    { label: i18n("projects.summary.overdue"), value: summary?.overdue_count, danger: true },
    { label: i18n("projects.summary.today"), value: i18n("projects.summary.placeholder") },
  ]

  return (
    <section className={styles.page} aria-labelledby="projects-heading">
      <header className={styles.header}>
        <div>
          <h1 className="text-page-title" id="projects-heading">{i18n("page.projects.title")}</h1>
          <p className="text-description">{i18n("projects.description")}</p>
        </div>
        <button
          className={`${controls.button} ${controls.primaryButton} ${controls.focusRing} text-button`}
          type="button"
          onClick={() => navigate("/projects/new")}
        >
          {i18n("projects.create")}
        </button>
      </header>

      <nav className={styles.tabs} aria-label={i18n("projects.status.select", { title: i18n("page.projects.title") })}>
        {projectTabs.map((tab) => {
          const count = tab === "trash" ? summary?.trash_count : summary?.status_counts[tab]
          return (
            <button
              key={tab}
              className={`${styles.tab} ${state.tab === tab ? styles.activeTab : ""} text-body`}
              type="button"
              aria-current={state.tab === tab ? "page" : undefined}
              onClick={() => selectTab(tab)}
            >
              <span>{i18n(projectTabLabels[tab])}</span>
              <span className={`${styles.tabCount} text-numeric`}>{count ?? "—"}</span>
            </button>
          )
        })}
      </nav>

      <section className={`${controls.card} ${styles.listCard}`} aria-label={i18n(projectTabLabels[state.tab])}>
        <div className={styles.summary}>
          {summaryValues.map(({ label, value, danger }) => (
            <div className={styles.summaryItem} key={label}>
              <span className="text-label">{label}</span>
              <strong className={`${danger ? styles.summaryDanger : ""} text-stat-value`}>
                {typeof value === "number"
                  ? i18n("projects.count", { count: new Intl.NumberFormat(language === "ja" ? "ja-JP" : "en-US").format(value) })
                  : value ?? "—"}
              </strong>
            </div>
          ))}
        </div>

        {query.isLoading ? (
          <p className={`${styles.message} text-body`} role="status">{i18n("projects.loading")}</p>
        ) : query.error && !query.data ? (
          <div className={styles.message} role="alert">
            <p className="text-body">{i18n("projects.loadError")}</p>
            <button className={`${controls.button} ${controls.outlineButton} ${controls.focusRing} text-button-small`} type="button" onClick={() => query.refetch()}>
              {i18n("projects.retry")}
            </button>
          </div>
        ) : !query.data?.items.length ? (
          <p className={`${styles.message} text-body`}>{i18n("projects.empty")}</p>
        ) : (
          <ProjectTable
            projects={query.data.items}
            tab={state.tab}
            sortColumn={state.sortColumn}
            sortOrder={state.sortOrder}
            statusOptions={statusOptions}
            busy={busy}
            onSortChange={changeSort}
            onStatusChange={changeStatus}
            onMoveToTrash={setProjectToTrash}
            onRestore={(project) => restoreMutation.mutate({ project })}
            onRowClick={handleRowClick}
          />
        )}

        <footer className={styles.pagination}>
          {state.pageToken && query.data?.items.length === 0 && !query.data.previous_page_token && (
            <button
              className={`${controls.button} ${controls.neutralButton} ${controls.focusRing} text-body`}
              type="button"
              onClick={() => setSearchParams((current) => setProjectPage(current), { preventScrollReset: true })}
            >
              {i18n("projects.firstPage")}
            </button>
          )}
          <button
            className={`${controls.button} ${controls.neutralButton} ${controls.focusRing} text-body`}
            type="button"
            disabled={!query.data?.previous_page_token || busy || query.isLoading}
            onClick={() => setSearchParams(setProjectPage(searchParams, query.data?.previous_page_token), { preventScrollReset: true })}
          >
            {i18n("projects.previous")}
          </button>
          <button
            className={`${controls.button} ${controls.neutralButton} ${controls.focusRing} text-body`}
            type="button"
            disabled={!query.data?.next_page_token || busy || query.isLoading}
            onClick={() => setSearchParams(setProjectPage(searchParams, query.data?.next_page_token), { preventScrollReset: true })}
          >
            {i18n("projects.next")}
          </button>
        </footer>
      </section>

      <ConfirmationDialog
        open={projectToTrash !== null}
        title={i18n("projects.dialog.trashTitle")}
        description={i18n("projects.dialog.trashDescription")}
        confirmLabel={i18n("projects.moveToTrash", { title: projectToTrash?.title ?? "" })}
        cancelLabel={i18n("projects.dialog.keepEditing")}
        busy={trashMutation.isPending}
        onConfirm={() => projectToTrash && trashMutation.mutate({ project: projectToTrash })}
        onCancel={() => setProjectToTrash(null)}
      />
    </section>
  )
}
