import { useContext, useEffect } from "react"
import {
  createColumnHelper,
  rowSortingFeature,
  tableFeatures,
  useTable,
} from "@tanstack/react-table"
import type { SortingState } from "@tanstack/react-table"
import type { RestProjectResponse } from "~/api/generated/projects"
import { useI18n, useLanguage } from "~/features/i18n/hooks"
import type { MessageKey } from "~/features/i18n/hooks"
import { getProjectErrorMessageKey } from "~/features/project/errors"
import { ProjectAPIDataContext } from "~/features/project/providers/ProjectAPIDataProvider/context"
import { ProjectActionContext } from "~/features/project/providers/ProjectActionProvider/context"
import { ProjectNavigationContext } from "~/features/project/providers/ProjectNavigationProvider/context"
import { notify } from "~/features/shared/notification"
import type { ProjectSortColumn } from "~/features/project/providers/ProjectNavigationProvider/context"
import ProjectStatusControl from "./parts/ProjectStatusControl"
import ProjectRowActions from "./parts/ProjectRowActions"
import controls from "~/styles/controls.module.css"
import styles from "./index.module.css"

const projectTableFeatures = tableFeatures({ rowSortingFeature })
const projectColumn = createColumnHelper<typeof projectTableFeatures, RestProjectResponse>()
const projectColumns = projectColumn.columns([
  projectColumn.accessor("title", { header: "projects.column.project", sortDescFirst: false }),
  projectColumn.accessor("progress", { header: "projects.column.progress", sortDescFirst: false }),
  projectColumn.accessor("end_date", { header: "projects.column.deadline", sortDescFirst: false }),
  projectColumn.accessor("remaining_days", { header: "projects.column.remaining", sortDescFirst: false }),
  projectColumn.display({ id: "todayTasks", header: "projects.column.todayTasks", enableSorting: false }),
  projectColumn.display({ id: "status", header: "projects.column.status", enableSorting: false }),
  projectColumn.display({ id: "actions", header: "projects.column.actions", enableSorting: false }),
])
const emptyProjects: RestProjectResponse[] = []

export default function ProjectTable() {
  const i18n = useI18n()
  const { language } = useLanguage()
  const { projectsQuery, optionsQuery } = useContext(ProjectAPIDataContext)!
  const { state, changeSort } = useContext(ProjectNavigationContext)!
  const { retry } = useContext(ProjectActionContext)!

  const listErrorKey = projectsQuery.error ? getProjectErrorMessageKey(projectsQuery.error) : undefined
  const optionsErrorKey = projectsQuery.isSuccess && optionsQuery.error
    ? getProjectErrorMessageKey(optionsQuery.error)
    : undefined
  useEffect(() => {
    if (listErrorKey) notify.error(i18n(listErrorKey))
  }, [i18n, listErrorKey])
  useEffect(() => {
    if (optionsErrorKey) notify.error(i18n(optionsErrorKey))
  }, [i18n, optionsErrorKey])

  const listError = projectsQuery.isError && !projectsQuery.data
  const listEmpty = !projectsQuery.isLoading && !listError && !projectsQuery.data?.items.length
  const sorting: SortingState = [{ id: state.sortColumn, desc: state.sortOrder === "desc" }]
  const table = useTable({
    features: projectTableFeatures,
    columns: projectColumns,
    data: projectsQuery.data?.items ?? emptyProjects,
    getRowId: (project) => project.id,
    state: { sorting },
    onSortingChange: (updater) => {
      const next = typeof updater === "function" ? updater(sorting) : updater
      const selected = next[0]
      if (!selected || !isProjectSortColumn(selected.id)) return
      changeSort(selected.id, selected.desc ? "desc" : "asc")
    },
    manualSorting: true,
    enableMultiSort: false,
    enableSortingRemoval: false,
  })

  if (projectsQuery.isLoading) return <Loading />
  if (listError) return <Alert onRetry={() => void retry()} />
  if (listEmpty) return <Empty />

  function renderCell(columnId: string, project: RestProjectResponse) {
    if (columnId === "title") {
      return (
        <div className={styles.projectName}>
          <strong className="text-body-strong">{project.title}</strong>
          {project.goal && <span className="text-caption">{project.goal}</span>}
        </div>
      )
    }
    if (columnId === "progress") {
      return (
        <div className={styles.progressCell}>
          <div className={styles.progressTrack} role="progressbar" aria-label={i18n("projects.column.progress")} aria-valuemin={0} aria-valuemax={100} aria-valuenow={project.progress}>
            <span className={styles.progressFill} style={{ width: `${Math.min(100, Math.max(0, project.progress))}%` }} />
          </div>
          <span className="text-numeric">{project.progress}%</span>
        </div>
      )
    }
    if (columnId === "end_date") return displayDate(project.end_date, language, i18n)
    if (columnId === "remaining_days") {
      if (project.remaining_days === null) return "—"
      if (project.remaining_days < 0) {
        return <span className={styles.overdue}>{i18n("projects.remaining.overdue", { days: Math.abs(project.remaining_days) })}</span>
      }
      if (project.remaining_days === 0) return <span className={styles.today}>{i18n("projects.remaining.today")}</span>
      return i18n("projects.remaining.days", { days: project.remaining_days })
    }
    if (columnId === "todayTasks") return i18n("projects.table.placeholder")
    if (columnId === "status") return <ProjectStatusControl project={project} />
    if (columnId === "actions") return <ProjectRowActions project={project} />
    return null
  }

  return (
    <div className={styles.tableScroll} role="region" aria-label={i18n("page.projects.title")} tabIndex={0}>
      <table className={styles.table}>
        <caption className={styles.srOnly}>{i18n("page.projects.title")}</caption>
        <thead>
          {table.getHeaderGroups().map((group) => (
            <tr key={group.id}>
              {group.headers.map((header) => {
                const direction = header.column.getIsSorted()
                const label = header.column.columnDef.header
                return (
                  <th key={header.id} scope="col" aria-sort={direction === "asc" ? "ascending" : direction === "desc" ? "descending" : "none"}>
                    {header.isPlaceholder ? null : header.column.getCanSort() ? (
                      <button
                        className={`${styles.sortButton} ${controls.focusRing} text-label`}
                        type="button"
                        onClick={header.column.getToggleSortingHandler()}
                      >
                        {typeof label === "string" ? i18n(label as MessageKey) : ""}
                        <span aria-hidden="true">{direction === "asc" ? "↑" : direction === "desc" ? "↓" : "↕"}</span>
                      </button>
                    ) : typeof label === "string" ? i18n(label as MessageKey) : null}
                  </th>
                )
              })}
            </tr>
          ))}
        </thead>
        <tbody>
          {table.getRowModel().rows.map((row) => (
            <tr key={row.id}>
              {row.getAllCells().map((cell) => (
                <td key={cell.id} className={cell.column.id === "progress" || cell.column.id === "remaining_days" || cell.column.id === "todayTasks" ? styles.numeric : undefined}>
                  {renderCell(cell.column.id, row.original)}
                </td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}

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

function isProjectSortColumn(value: string): value is ProjectSortColumn {
  return value === "title" || value === "progress" || value === "end_date" || value === "remaining_days"
}

function displayDate(date: string | null, language: string, format: (key: MessageKey) => string) {
  if (!date) return format("projects.deadline.none")
  const value = new Date(`${date}T00:00:00.000Z`)
  return new Intl.DateTimeFormat(language === "ja" ? "ja-JP" : "en-US", {
    year: "numeric",
    month: "short",
    day: "numeric",
    timeZone: "UTC",
  }).format(value)
}
