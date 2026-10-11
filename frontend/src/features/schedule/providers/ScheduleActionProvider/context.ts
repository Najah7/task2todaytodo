import { createContext } from "react"
import type { ScheduleFormValues } from "~/features/schedule/components/ScheduleForm/schema"
import type { ScheduleEvent } from "~/features/schedule/providers/ScheduleAPIDataProvider/context"

export type ScheduleScope = "current" | "following"

export type ScheduleEditInput = {
  blueprintId: string
  occurrenceDate: string
  revision: number
  blueprintRevision: number
  scope: ScheduleScope
  values: ScheduleFormValues
  baselineValues: ScheduleFormValues
}

export type ScheduleReloadSnapshot = {
  values: ScheduleFormValues
  occurrenceRevision: number
  blueprintRevision?: number
  followingAvailable: boolean
}

export type ScheduleActions = {
  busy: boolean
  create: (values: ScheduleFormValues) => Promise<void>
  update: (input: ScheduleEditInput) => Promise<void>
  complete: (event: ScheduleEvent) => Promise<void>
  reopen: (event: ScheduleEvent) => Promise<void>
  skip: (event: ScheduleEvent) => Promise<void>
  restore: (event: ScheduleEvent) => Promise<void>
  delete: (event: ScheduleEvent, scope: ScheduleScope) => Promise<void>
  reloadLatest: (event: ScheduleEvent, scope: ScheduleScope, currentValues?: ScheduleFormValues) => Promise<ScheduleReloadSnapshot>
}

export const ScheduleActionContext = createContext<ScheduleActions | null>(null)
