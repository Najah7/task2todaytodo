import type { ReactNode } from "react"
import type { Tab } from "~/store/tab"

const icons: Record<Tab, ReactNode> = {
  today: <><circle cx="12" cy="12" r="4" /><path d="M12 2v2m0 16v2M2 12h2m16 0h2M5 5l1.5 1.5m11 11L19 19M5 19l1.5-1.5m11-11L19 5" /></>,
  inbox: <><path d="m3 13 3-8h12l3 8v6H3z" /><path d="M3 13h5l2 3h4l2-3h5" /></>,
  projects: <path d="M3 7V5h6l2 2h10v13H3z" />,
  tasks: <path d="m3 6 2 2 3-3m-5 7 2 2 3-3m-5 7 2 2 3-3M11 6h10M11 12h10M11 18h10" />,
  calendar: <><rect x="3" y="5" width="18" height="16" rx="2" /><path d="M7 3v4m10-4v4M3 10h18" /></>,
  kpi: <path d="M3 3v18h18M7 17v-6m5 6V6m5 11V9" />,
  profile: <><circle cx="12" cy="7" r="4" /><path d="M4 21v-2a8 8 0 0 1 16 0v2" /></>,
}

export default function MenuIcon({ tab }: { tab: Tab }) {
  return (
    <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true" focusable="false">
      {icons[tab]}
    </svg>
  )
}
