import { useI18n } from "~/features/i18n/hooks"
import type { TaskFilterView } from "~/features/task/types"
import { useTaskListData } from "~/features/task/providers/TaskList/data"
import { useTaskListNavigation } from "~/features/task/providers/TaskList/navigation"
import controls from "~/styles/controls.module.css"
import styles from "./index.module.css"

export default function TaskListToolbar() {
  const i18n = useI18n()
  const { filters, sort, sortOptions } = useTaskListData()
  const { changeFilter, changeSort } = useTaskListNavigation()
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
