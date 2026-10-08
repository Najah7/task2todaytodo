import {
  createColumnHelper,
  rowSortingFeature,
  tableFeatures,
  useTable,
} from "@tanstack/react-table"
import type { SortingState } from "@tanstack/react-table"
import { useNavigate } from "react-router"
import type {
  GetProjectsSortOrder,
  GetProjectsStatus as ProjectStatus,
  RestProjectResponse,
  RestProjectTaskStatusResponse,
} from "~/api/generated/projects"
import { useI18n, useLanguage } from "~/features/i18n/hooks"
import type { MessageKey } from "~/features/i18n/hooks"
import { projectTabLabels, type ProjectSortColumn, type ProjectTab } from "~/features/project/listState"
import SeachSelect from "~/features/shared/components/SeachSelect"
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

type Props = {
  projects: RestProjectResponse[]
  tab: ProjectTab
  sortColumn: ProjectSortColumn
  sortOrder: GetProjectsSortOrder
  statusOptions: RestProjectTaskStatusResponse[]
  busy: boolean
  onSortChange: (column: ProjectSortColumn, order: GetProjectsSortOrder) => void
  onStatusChange: (project: RestProjectResponse, value: string) => void
  onMoveToTrash: (project: RestProjectResponse) => void
  onRestore: (project: RestProjectResponse) => void
  onRowClick: (project: RestProjectResponse) => void
}

export default function ProjectTable({
  projects,
  tab,
  sortColumn,
  sortOrder,
  statusOptions,
  busy,
  onSortChange,
  onStatusChange,
  onMoveToTrash,
  onRestore,
  onRowClick,
}: Props) {
  const i18n = useI18n()
  const { language } = useLanguage()
  const navigate = useNavigate()
  const sorting: SortingState = [{ id: sortColumn, desc: sortOrder === "desc" }]
  const table = useTable({
    features: projectTableFeatures,
    columns: projectColumns,
    data: projects ?? emptyProjects,
    getRowId: (project) => project.id,
    state: { sorting },
    onSortingChange: (updater) => {
      const next = typeof updater === "function" ? updater(sorting) : updater
      const selected = next[0]
      if (!selected || !isProjectSortColumn(selected.id)) return
      onSortChange(selected.id, selected.desc ? "desc" : "asc")
    },
    manualSorting: true,
    enableMultiSort: false,
    enableSortingRemoval: false,
  })

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
    if (columnId === "status") {
      if (tab === "trash") return projectRowLabel(statusOptions.find((status) => status.value === project.status), language, i18n(projectTabLabels[project.status]))
      return (
        <SeachSelect
          ariaLabel={i18n("projects.status.select", { title: project.title })}
          className={styles.statusSelect}
          disabled={!project.can_update || busy || statusOptions.length === 0}
          value={project.status}
          options={statusOptions.flatMap((option) => option.value ? [{
            value: option.value,
            label: projectRowLabel(option, language, i18n(projectTabLabels[option.value as ProjectStatus] ?? "projects.status.open")),
          }] : [])}
          onChange={(value) => onStatusChange(project, value)}
        />
      )
    }
    if (columnId === "actions") {
      return (
        <div className={styles.actions} onClick={(event) => event.stopPropagation()}>
          {tab === "trash" ? (
            project.can_delete && (
              <button
                className={`${controls.button} ${controls.outlineButton} ${controls.focusRing} text-button-small`}
                type="button"
                disabled={busy}
                onClick={() => onRestore(project)}
              >
                {i18n("projects.restore")}
              </button>
            )
          ) : (
            <>
              {project.can_update && (
                <button
                  className={`${controls.button} ${controls.neutralButton} ${controls.focusRing} text-button-small`}
                  type="button"
                  disabled={busy}
                  onClick={() => navigate(`/projects/${encodeURIComponent(project.id)}/edit`)}
                >
                  {i18n("projects.edit")}
                </button>
              )}
              {project.can_delete && (
                <button
                  className={`${styles.iconButton} ${controls.focusRing}`}
                  type="button"
                  aria-label={i18n("projects.moveToTrash", { title: project.title })}
                  disabled={busy}
                  onClick={() => onMoveToTrash(project)}
                >
                  <svg viewBox="0 0 24 24" aria-hidden="true" focusable="false">
                    <path d="M4 7h16M9 7V4h6v3m3 0-.8 13H6.8L6 7m4 4v5m4-5v5" />
                  </svg>
                </button>
              )}
            </>
          )}
        </div>
      )
    }
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
            <tr key={row.id} onClick={tab === "trash" ? undefined : () => onRowClick(row.original)}>
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

function isProjectSortColumn(value: string): value is ProjectSortColumn {
  return value === "title" || value === "progress" || value === "end_date" || value === "remaining_days"
}

function projectRowLabel(option: RestProjectTaskStatusResponse | undefined, language: string, fallback: string) {
  if (!option) return fallback
  return language === "ja" ? option.label_jp || option.label || fallback : option.label || option.label_jp || fallback
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
