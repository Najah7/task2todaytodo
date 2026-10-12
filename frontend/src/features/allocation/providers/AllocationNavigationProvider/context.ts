import { createContext } from "react"

export type AllocationNavigation = {
  weekStart: string
  changeWeek: (weekStart: string) => void
  editPlan: () => void
  backToPlan: () => void
}

export const AllocationNavigationContext = createContext<AllocationNavigation | null>(null)
