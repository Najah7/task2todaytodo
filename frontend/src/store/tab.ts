export const tabs = ["today", "inbox", "projects", "tasks", "calendar", "kpi", "profile"] as const

export type Tab = (typeof tabs)[number]

export const DEFAULT_TAB: Tab = "today"
