import { useContext } from "react"
import { useI18n } from "~/features/i18n/hooks"
import { TaskAPIDataContext } from "~/features/task/providers/TaskAPIDataProvider/context"
import { TaskNavigationContext, type TaskFilterView } from "~/features/task/providers/TaskNavigationProvider/context"
import { GetTasksDueFilter } from "~/api/generated/tasks"
import controls from "~/styles/controls.module.css"
import styles from "./index.module.css"

export default function TaskListToolbar() {
  const i18n = useI18n()
  const { projectsQuery } = useContext(TaskAPIDataContext)!
  const { state, changeFilter, changeSort } = useContext(TaskNavigationContext)!
  const filters = {
    projectOptions: [
      { value: "", label: i18n("tasks.filter.allProjects") },
      ...(projectsQuery.data ?? []).map((project) => ({ value: project.id, label: project.title })),
    ],
    dueOptions: [
      { value: GetTasksDueFilter.all, label: i18n("tasks.filter.due.all") },
      { value: GetTasksDueFilter.overdue, label: i18n("tasks.filter.due.overdue") },
      { value: GetTasksDueFilter.today, label: i18n("tasks.filter.due.today") },
      { value: GetTasksDueFilter.due_soon, label: i18n("tasks.filter.due.dueSoon") },
      { value: GetTasksDueFilter.no_due, label: i18n("tasks.filter.due.noDue") },
    ],
    values: { projectId: state.projectId ?? "", dueFilter: state.dueFilter ?? GetTasksDueFilter.all, title: state.title },
  }
  const sort = `${state.sortBy}:${state.sortOrder}`
  const sortOptions = [
    { value: "due_date:asc", label: i18n("tasks.sort.dueAsc") },
    { value: "due_date:desc", label: i18n("tasks.sort.dueDesc") },
    { value: "title:asc", label: i18n("tasks.sort.titleAsc") },
    { value: "created_at:desc", label: i18n("tasks.sort.createdDesc") },
  ]
  function updateFilter<K extends keyof TaskFilterView>(key: K, value: TaskFilterView[K]) {
    changeFilter({ ...filters.values, [key]: value })
  }
  return (
    <div className={styles.toolbar}>
      <div className={styles.filters}>
        <label className={styles.filter}>
          <span className="text-label">{i18n("tasks.filter.project")}</span>
          <select className={`${controls.select} ${styles.select} text-body`} value={filters.values.projectId} onChange={(event) => updateFilter("projectId", event.target.value)}>
            {filters.projectOptions.map((option) => <option key={option.value} value={option.value}>{option.label}</option>)}
          </select>
        </label>
        <label className={styles.filter}>
          <span className="text-label">{i18n("tasks.filter.due")}</span>
          <select className={`${controls.select} ${styles.select} text-body`} value={filters.values.dueFilter} onChange={(event) => updateFilter("dueFilter", event.target.value)}>
            {filters.dueOptions.map((option) => <option key={option.value} value={option.value}>{option.label}</option>)}
          </select>
        </label>
        <label className={`${styles.filter} ${styles.titleFilter}`}>
          <span className={styles.srOnly}>{i18n("tasks.filter.title")}</span>
          <input className={`${controls.input} ${styles.titleInput} text-body`} type="search" value={filters.values.title} placeholder={i18n("tasks.filter.title")} onChange={(event) => updateFilter("title", event.target.value)} />
        </label>
      </div>
      <label className={styles.sort}>
        <span className="text-label">{i18n("tasks.sort.label")}</span>
        <select className={`${controls.select} ${styles.select} text-body`} value={sort} onChange={(event) => changeSort(event.target.value)}>
          {sortOptions.map((option) => <option key={option.value} value={option.value}>{option.label}</option>)}
        </select>
      </label>
    </div>
  )
}
