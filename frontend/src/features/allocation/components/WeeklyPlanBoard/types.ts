export type WeeklyPlanProjectView = {
  id: string
  title: string
  targetMinutes: number
  allocatedMinutes: number
  differenceMinutes: number
  colorIndex: number
}

export type WeeklyPlanDayView = {
  date: string
  allocatedMinutes: number
  freeMinutes: number
  wakeAt: string
  bedtimeAt: string
  hasSleepException: boolean
  sleepRevision: number
}

export type WeeklyPlanAllocationView = {
  id: string
  ruleId: string
  occurrenceDate: string
  revision: number
  ruleRevision: number
  timezone: string
  startAt: string
  endAt: string
  projectId: string
  projectTitle: string
  colorIndex: number
  date: string
  startMinute: number
  endMinute: number
  startLabel: string
  endLabel: string
  canUpdate: boolean
  canDelete: boolean
  canUpdateFollowing: boolean
  canDeleteFollowing: boolean
}

export type WeeklyPlanSleepView = {
  date: string
  startMinute: number
  endMinute: number
}

export type WeeklyPlanPausedRuleView = {
  ruleId: string
  projectId: string
  projectTitle: string
  reason: string
}

export type WeeklyPlanBoardProps = {
  weekStart: string
  timezone: string
  projects: WeeklyPlanProjectView[]
  days: WeeklyPlanDayView[]
  allocations: WeeklyPlanAllocationView[]
  sleepIntervals: WeeklyPlanSleepView[]
  pausedRules: WeeklyPlanPausedRuleView[]
  allocatedMinutes: number
  targetMinutes: number
  differenceMinutes: number
  freeMinutes: number
  canEdit: boolean
  busy: boolean
  onChangeWeek: (weekStart: string) => void
  onEditPlan: () => void
  onCreateAllocation: (projectId: string, date: string, startMinute: number, endMinute: number) => void
  onEditAllocation: (allocation: WeeklyPlanAllocationView) => void
  onEditSleep: (date: string) => void
}
