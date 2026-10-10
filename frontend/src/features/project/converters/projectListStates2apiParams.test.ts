import { expect, test } from "vitest"
import { GetProjectsStatus } from "~/api/generated/projects"
import type { ProjectListState } from "~/features/project/providers/ProjectNavigationProvider/context"
import { projectListStates2apiParams } from "./projectListStates2apiParams"

test("converts a collection of project list states to API parameters", () => {
  const states: ProjectListState[] = [
    { tab: GetProjectsStatus.in_progress, sortColumn: "end_date", sortOrder: "asc" },
    { tab: "trash", sortColumn: "remaining_days", sortOrder: "desc", pageToken: "next" },
  ]

  expect(projectListStates2apiParams(states)).toEqual([
    { status: "in_progress", sort_by: "end_date", sort_order: "asc", page_size: 20 },
    { view: "trash", sort_by: "end_date", sort_order: "desc", page_size: 20, page_token: "next" },
  ])
})
