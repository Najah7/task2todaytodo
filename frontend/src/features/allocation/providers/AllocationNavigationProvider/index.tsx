import type { ReactNode } from "react"
import { useNavigate, useSearchParams } from "react-router"
import { AllocationNavigationContext } from "./context"

export function AllocationNavigationProvider({ weekStart, children }: { weekStart: string; children: ReactNode }) {
  const navigate = useNavigate()
  const [searchParams, setSearchParams] = useSearchParams()
  const value = {
    weekStart,
    changeWeek: (nextWeek: string) => {
      const next = new URLSearchParams(searchParams)
      next.set("week_start", nextWeek)
      setSearchParams(next, { preventScrollReset: true })
    },
    editPlan: () => navigate("/weekly-plan/edit?week_start=" + encodeURIComponent(weekStart)),
    backToPlan: () => navigate("/weekly-plan?week_start=" + encodeURIComponent(weekStart)),
  }
  return <AllocationNavigationContext.Provider value={value}>{children}</AllocationNavigationContext.Provider>
}
