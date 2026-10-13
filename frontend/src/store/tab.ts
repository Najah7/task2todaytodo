export const tabs = ["today", "inbox", "projects", "tasks", "schedules", "weekly-plan", "members", "metrics", "profile"] as const

export type Tab = (typeof tabs)[number]

export const DEFAULT_TAB: Tab = "inbox"
