import { describe, expect, it } from "vitest"
import { inboxListChanges2searchParams, inboxPageTokenChanges2searchParams, inboxSearchParams2state, inboxState2apiParams } from "~/features/inbox/converters/inboxSearchParams"

describe("inbox search parameter conversion", () => {
  it("normalizes type and project filters and preserves title and page token", () => {
    const state = inboxSearchParams2state([new URLSearchParams("type=schedule&project_id=no-project&title=travel&page_token=next")])[0]!
    expect(state).toEqual({ type: "schedule", projectId: "no-project", title: "travel", pageToken: "next" })
    expect(inboxState2apiParams([state])).toEqual([{ type: "schedule", project_id: "no-project", title: "travel", page_size: 50, page_token: "next" }])
  })

  it("defaults invalid and missing filters to all", () => {
    expect(inboxSearchParams2state([new URLSearchParams("type=other&project_id=")])).toEqual([{ type: "all", projectId: "all", title: "", pageToken: undefined }])
  })

  it("clears cursor when filters change and keeps other URL state", () => {
    const [updated] = inboxListChanges2searchParams([{
      searchParams: new URLSearchParams("type=task&page_token=old&keep=1"),
      type: "all", projectId: "project-1", title: "Plan",
    }])
    expect(updated.toString()).toBe("keep=1&project_id=project-1&title=Plan")
  })

  it("preserves other active filters when changing only one filter", () => {
    const current = new URLSearchParams("type=schedule&project_id=project-1&title=Trip&page_token=old")
    const [typeChange] = inboxListChanges2searchParams([{ searchParams: current, type: "task" }])
    const [projectChange] = inboxListChanges2searchParams([{ searchParams: current, projectId: "no-project" }])
    const [titleChange] = inboxListChanges2searchParams([{ searchParams: current, title: "Work" }])

    expect(typeChange.toString()).toBe("type=task&project_id=project-1&title=Trip")
    expect(projectChange.toString()).toBe("type=schedule&project_id=no-project&title=Trip")
    expect(titleChange.toString()).toBe("type=schedule&project_id=project-1&title=Work")
  })

  it("changes page token without dropping filters", () => {
    const [updated] = inboxPageTokenChanges2searchParams([{
      searchParams: new URLSearchParams("type=task&title=Plan&page_token=old"), pageToken: "next",
    }])
    expect(updated.toString()).toBe("type=task&title=Plan&page_token=next")
  })
})
