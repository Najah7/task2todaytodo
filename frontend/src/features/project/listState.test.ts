import { expect, test } from "vitest"
import {
  getProjectListParams,
  readProjectListState,
  setProjectPage,
  setProjectSort,
  setProjectTab,
} from "./listState"

test("defaults to the in-progress tab, end-date ascending, and a 20-row server page", () => {
  const state = readProjectListState(new URLSearchParams())
  expect(state).toEqual({ tab: "in_progress", sortColumn: "end_date", sortOrder: "asc", pageToken: undefined })
  expect(getProjectListParams(state)).toEqual({
    status: "in_progress",
    sort_by: "end_date",
    sort_order: "asc",
    page_size: 20,
  })
})

test("preserves sort state across tabs and maps remaining-days sort to server end-date order", () => {
  const search = new URLSearchParams("status=pending&sort_by=remaining_days&sort_order=desc&page_token=cursor")
  const state = readProjectListState(search)
  const next = setProjectTab(search, "trash")
  expect(readProjectListState(next)).toMatchObject({ tab: "trash", sortColumn: "remaining_days", sortOrder: "desc", pageToken: undefined })
  expect(getProjectListParams(readProjectListState(next))).toEqual({ view: "trash", sort_by: "end_date", sort_order: "desc", page_size: 20 })

  const active = setProjectTab(next, "done")
  expect(active.get("view")).toBeNull()
  expect(active.get("status")).toBe("done")
  expect(readProjectListState(active).sortColumn).toBe(state.sortColumn)
})

test("sort changes and page changes retain their URL state while sort starts at the first page", () => {
  const search = new URLSearchParams("status=open&page_token=cursor")
  const sorted = setProjectSort(search, "title", "desc")
  expect(sorted.get("page_token")).toBeNull()
  expect(readProjectListState(sorted)).toMatchObject({ tab: "open", sortColumn: "title", sortOrder: "desc" })
  expect(setProjectPage(sorted, "next-cursor").get("page_token")).toBe("next-cursor")
  expect(setProjectPage(sorted, undefined).get("page_token")).toBeNull()
})
