export const tabs = ["today", "inbox", "projects", "tasks", "schedules", "members", "kpi", "profile"] as const

export type Tab = (typeof tabs)[number]

export const DEFAULT_TAB: Tab = "today"
