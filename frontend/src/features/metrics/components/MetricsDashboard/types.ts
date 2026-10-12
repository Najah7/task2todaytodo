export type MetricsDashboardProps = {
  period: 7 | 30
  plannedItems: number
  completedItems: number
  completionRatio: number | null
  overdueCount: number
  isPartialToday: boolean
  days: MetricsDay[]
  overdueGroups: OverdueGroup[]
  otherOverdueCount: number
  hasPlannedWork: boolean
  onPeriodChange: (period: 7 | 30) => void
}

export type MetricsDay = {
  date: string
  plannedItems: number
  completedItems: number
  completionRatio: number | null
}

export type OverdueGroup = {
  id: string
  title: string
  count: number
}
