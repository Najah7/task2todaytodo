import { expect, test } from "vitest"
import { GetProjectsStatus } from "~/api/generated/projects"
import { projectPageTokenChanges2searchParams } from "./projectPageTokenChanges2searchParams"
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
