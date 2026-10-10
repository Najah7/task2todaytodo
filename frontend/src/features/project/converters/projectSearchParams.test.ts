import { expect, test } from "vitest"
import { GetProjectsStatus } from "~/api/generated/projects"
import { projectPageTokenChanges2searchParams } from "./projectPageTokenChanges2searchParams"
import { projectSearchParams2projectListStates } from "./projectSearchParams2projectListStates"
import { projectSortChanges2searchParams } from "./projectSortChanges2searchParams"
import { projectTabChanges2searchParams } from "./projectTabChanges2searchParams"

test("updates tab and sort search parameters and clears the page token", () => {
  const searchParams = new URLSearchParams("status=pending&page_token=cursor")
  const [trashParams] = projectTabChanges2searchParams([{ searchParams, tab: "trash" }])
  expect(trashParams?.get("status")).toBeNull()
  expect(trashParams?.get("view")).toBe("trash")
  expect(trashParams?.get("page_token")).toBeNull()

  const [sortedParams] = projectSortChanges2searchParams([{
    searchParams: new URLSearchParams("status=open&page_token=cursor"),
    sortColumn: "title",
    sortOrder: "desc",
  }])
  expect(sortedParams?.get("status")).toBe("open")
  expect(sortedParams?.get("sort_by")).toBe("title")
  expect(sortedParams?.get("sort_order")).toBe("desc")
  expect(sortedParams?.get("page_token")).toBeNull()
})

test("updates or clears page tokens while preserving other parameters", () => {
  const [nextParams] = projectPageTokenChanges2searchParams([{
    searchParams: new URLSearchParams(`status=${GetProjectsStatus.done}&sort_by=title`),
    pageToken: "next-cursor",
  }])
  expect(nextParams?.get("page_token")).toBe("next-cursor")
  expect(nextParams?.get("status")).toBe(GetProjectsStatus.done)

  const [firstPageParams] = projectPageTokenChanges2searchParams([{ searchParams: nextParams! }])
  expect(firstPageParams?.get("page_token")).toBeNull()
})

test("parses URL parameters into list state with safe defaults", () => {
  const [trashState, defaultState] = projectSearchParams2projectListStates([
    new URLSearchParams("view=trash&sort_by=title&sort_order=desc&page_token=cursor"),
    new URLSearchParams("status=invalid&sort_by=unknown&sort_order=unknown"),
  ])

  expect(trashState).toEqual({ tab: "trash", sortColumn: "title", sortOrder: "desc", pageToken: "cursor" })
  expect(defaultState).toEqual({ tab: GetProjectsStatus.in_progress, sortColumn: "end_date", sortOrder: "asc", pageToken: undefined })
})
