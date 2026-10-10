/* oxlint-disable react/only-export-components -- story helper exports provider and decorator for feature stories. */
import type { ReactNode } from "react"
import type { Decorator } from "@storybook/react-vite"
import type { RestProjectResponse } from "~/api/generated/projects"
import { ProjectListProvider } from "~/features/project/providers/ProjectList"
import type { ProjectListActions, ProjectListData, ProjectListNavigation } from "~/features/project/types"

const summary: NonNullable<ProjectListData["summary"]> = {
  due_soon_count: 2,
  overdue_count: 1,
  status_counts: { done: 8, in_progress: 3, open: 4, pending: 2, waiting_on_others: 1 },
  timezone: "Asia/Tokyo",
  today: "2026-10-10",
  total_count: 3,
  trash_count: 5,
}

const project: RestProjectResponse = {
  can_delete: true,
  can_update: true,
  created_at: 1_791_481_200,
  deleted_at: null,
  description: "Prepare the next release with the product team.",
  end_date: "2026-10-15",
  goal: "Ship the October release",
  id: "proj-october-release",
  priority: { label: "High", label_jp: "高", value: "high", weight: 3 },
  progress: 62,
  remaining_days: 6,
  revision: 3,
  start_date: "2026-09-01",
  status: "in_progress",
  title: "October release",
  type: { label: "Work", label_jp: "仕事", value: "work" },
  updated_at: 1_791_481_200,
  user_id: "user-story",
}

const dataDefaults: ProjectListData = {
  projects: [project],
  summary,
  statusOptions: [
    { label: "In progress", label_jp: "進行中", value: "in_progress" },
    { label: "Pending", label_jp: "保留", value: "pending" },
    { label: "Done", label_jp: "完了", value: "done" },
    { label: "Open", label_jp: "オープン", value: "open" },
    { label: "Waiting on others", label_jp: "他者待ち", value: "waiting_on_others" },
  ],
}

const navigationDefaults: ProjectListNavigation = {
  state: { tab: "in_progress", sortColumn: "end_date", sortOrder: "asc" },
  showFirstPage: false,
  previousDisabled: false,
  nextDisabled: false,
  selectTab: () => {},
  changeSort: () => {},
  onFirstPage: () => {},
  onPrevious: () => {},
  onNext: () => {},
}

const actionsDefaults: ProjectListActions = {
  busy: false,
  changeStatus: () => {},
  edit: () => {},
  moveToTrash: () => {},
  restore: () => {},
  rowClick: () => {},
}

type Overrides = {
  data?: Partial<ProjectListData>
  navigation?: Partial<Omit<ProjectListNavigation, "state">> & { state?: Partial<ProjectListNavigation["state"]> }
  actions?: Partial<ProjectListActions>
}

export function ProjectListStoryProvider({ children, data, navigation, actions }: Overrides & { children: ReactNode }) {
  return (
    <ProjectListProvider
      data={{ ...dataDefaults, ...data }}
      navigation={{ ...navigationDefaults, ...navigation, state: { ...navigationDefaults.state, ...navigation?.state } }}
      actions={{ ...actionsDefaults, ...actions }}
    >
      {children}
    </ProjectListProvider>
  )
}

export function withProjectListStoryContext(overrides: Overrides = {}): Decorator {
  return (Story) => (
    <ProjectListStoryProvider {...overrides}>
      <Story />
    </ProjectListStoryProvider>
  )
}
