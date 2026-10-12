import { createContext } from "react"

export type ScheduleView = "day" | "week" | "month"

export type ScheduleSlotPrefill = {
  startDate: string
  endDate: string
  startTime: string
  endTime: string
  allDay: boolean
}

export type ScheduleNavigationState = {
  date: string
  view: ScheduleView
  projectId?: string
  showSkipped: boolean
}

export type ScheduleNavigation = {
  state: ScheduleNavigationState
  slotPrefill?: ScheduleSlotPrefill
  setDate: (date: string, options?: { replace?: boolean }) => void
  setView: (view: ScheduleView, date?: string) => void
  openDay: (date: string) => void
  setProject: (projectId?: string) => void
  setShowSkipped: (show: boolean) => void
  previous: () => void
  next: () => void
  today: () => void
  create: (prefill?: ScheduleSlotPrefill) => void
  edit: (ruleId: string, occurrenceDate: string) => void
  list: () => void
  cancel: () => void
  afterSave: () => void
  setSlotPrefill: (prefill?: ScheduleSlotPrefill) => void
}

export const ScheduleNavigationContext = createContext<ScheduleNavigation | null>(null)
