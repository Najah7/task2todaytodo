import { createContext } from "react"
import type { UseQueryResult } from "@tanstack/react-query"

export type ScheduleAPIDataLoadState = "loading" | "error" | "forbidden" | "ready"
export type ScheduleFrequency = "once" | "daily" | "weekly" | "monthly" | "yearly"

export type ScheduleEventPermissions = {
  canUpdate: boolean
  canDelete: boolean
  canComplete: boolean
  canSkip: boolean
  canRestore: boolean
  canAssign: boolean
  canUpdateFollowing: boolean
  canDeleteFollowing: boolean
}

export type ScheduleEvent = {
  id: string
  blueprintId: string
  occurrenceDate: string
  title: string
  description: string
  location: string
  start: Date
  end: Date
  localStart: string
  localEnd: string
  startDate: string
  endDate: string
  timezone: string
  allDay: boolean
  projectId: string
  projectTitle: string
  assigneeId: string
  sourceRevisionId?: string
  revision: number
  blueprintRevision?: number
  state: "scheduled" | "completed" | "skipped"
  persisted: boolean
  permissions: ScheduleEventPermissions
  recurrence?: {
    frequency: ScheduleFrequency
    interval: number
    weekdays: string[]
    endDate: string
  }
}

export type ScheduleProjectOption = { id: string; title: string; ownerId?: string }
export type ScheduleAssigneeOption = { id: string; displayName: string }

export type ScheduleBlueprintRevision = {
  revisionId: string
  revision: number
  effectiveFrom: string
  effectiveUntil?: string
  superseded: boolean
  deleted: boolean
  title: string
  description: string
  location: string
  projectId: string
  assigneeId: string
  timezone: string
  allDay: boolean
  localStart: string
  localEnd: string
  frequency: ScheduleFrequency
  interval: number
  weekdays: string[]
  endDate: string
}

export type ScheduleAPIData = {
  state: ScheduleAPIDataLoadState
  list: boolean
  blueprintId?: string
  occurrenceDate?: string
  events: ScheduleEvent[]
  event?: ScheduleEvent
  blueprintRevision?: ScheduleBlueprintRevision
  blueprintRevisionNumber?: number
  blueprintAccessDenied: boolean
  projectOptions: ScheduleProjectOption[]
  assigneeOptions: ScheduleAssigneeOption[]
  assigneeOptionsProjectId?: string
  timezone: string
  userId: string
  userName: string
  timezones: string[]
  showSkipped: boolean
  scheduleQuery: UseQueryResult<unknown>
  blueprintQuery: UseQueryResult<unknown>
  occurrenceQuery: UseQueryResult<unknown>
  userQuery: UseQueryResult<unknown>
  timezonesQuery: UseQueryResult<unknown>
  refetch: () => Promise<void>
}

export const ScheduleAPIDataContext = createContext<ScheduleAPIData | null>(null)
