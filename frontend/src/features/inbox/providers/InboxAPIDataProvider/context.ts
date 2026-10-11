import { createContext } from "react"
import type { UseQueryResult } from "@tanstack/react-query"
import type { RestInboxListResponse } from "~/api/generated/inbox"
import type { RestProjectResponse } from "~/api/generated/projects"
import type { RestUserResponse } from "~/api/generated/auth"

export type InboxAPIDataLoadState = "loading" | "error" | "forbidden" | "ready"
export type InboxProjectOption = { id: string; title: string }

export type InboxAPIData = {
  state: InboxAPIDataLoadState
  inbox?: RestInboxListResponse
  inboxQuery: UseQueryResult<RestInboxListResponse>
  projects: InboxProjectOption[]
  projectsQuery: UseQueryResult<RestProjectResponse[]>
  user?: RestUserResponse
  userQuery: UseQueryResult<RestUserResponse>
  timezone: string
  userId: string
}

export const InboxAPIDataContext = createContext<InboxAPIData | null>(null)
