import { ListInboxType } from "~/api/generated/inbox"
import type { ListInboxParams, ListInboxType as InboxType } from "~/api/generated/inbox"

export type InboxListState = {
  type: InboxType
  projectId: string
  title: string
  pageToken?: string
}

const types = Object.values(ListInboxType)

export function inboxSearchParams2state(searches: URLSearchParams[]): InboxListState[] {
  return searches.map((search) => {
    const type = search.get("type")
    const projectId = search.get("project_id")
    return {
      type: types.includes(type as InboxType) ? type as InboxType : ListInboxType.all,
      projectId: projectId || "all",
      title: search.get("title") ?? "",
      pageToken: search.get("page_token") || undefined,
    }
  })
}

export function inboxState2apiParams(states: InboxListState[]): ListInboxParams[] {
  return states.map((state) => ({
    ...(state.type !== ListInboxType.all ? { type: state.type } : {}),
    ...(state.projectId !== "all" ? { project_id: state.projectId } : {}),
    ...(state.title ? { title: state.title } : {}),
    page_size: 50,
    ...(state.pageToken ? { page_token: state.pageToken } : {}),
  }))
}

export function inboxListChanges2searchParams(
  changes: { searchParams: URLSearchParams; type?: InboxType; projectId?: string; title?: string }[],
): URLSearchParams[] {
  return changes.map((change) => {
    const { searchParams, type, projectId, title } = change
    const next = new URLSearchParams(searchParams)
    next.delete("page_token")
    if ("type" in change) setOrDelete(next, "type", type && type !== ListInboxType.all ? type : undefined)
    if ("projectId" in change) setOrDelete(next, "project_id", projectId && projectId !== "all" ? projectId : undefined)
    if ("title" in change) setOrDelete(next, "title", title)
    return next
  })
}

export function inboxPageTokenChanges2searchParams(
  changes: { searchParams: URLSearchParams; pageToken?: string }[],
): URLSearchParams[] {
  return changes.map(({ searchParams, pageToken }) => {
    const next = new URLSearchParams(searchParams)
    setOrDelete(next, "page_token", pageToken)
    return next
  })
}

function setOrDelete(search: URLSearchParams, key: string, value?: string) {
  if (value) search.set(key, value)
  else search.delete(key)
}
