import type { Meta, StoryObj } from "@storybook/react-vite"
import type { ReactNode } from "react"
import { MemoryRouter } from "react-router"
import WeeklyPlanBoard from "./index"
import type { WeeklyPlanBoardProps } from "./types"

const dates = Array.from({ length: 7 }, (_, day) => `2026-10-${String(day + 12).padStart(2, "0")}`)
const args: WeeklyPlanBoardProps = {
  weekStart: "2026-10-12",
  timezone: "Asia/Tokyo",
  projects: [
    { id: "project-1", title: "Product research", targetMinutes: 600, allocatedMinutes: 360, differenceMinutes: -240, colorIndex: 1 },
    { id: "project-2", title: "Website refresh", targetMinutes: 480, allocatedMinutes: 240, differenceMinutes: -240, colorIndex: 2 },
  ],
  days: dates.map((date, index) => ({ date, allocatedMinutes: index === 0 ? 120 : 0, freeMinutes: 960, activeStartAt: "07:00", activeEndAt: "23:00", hasActiveException: false, activeRevision: 0 })),
  allocations: [{ id: "allocation-1:2026-10-12", ruleId: "rule-1", occurrenceDate: "2026-10-12", revision: 1, ruleRevision: 2, timezone: "Asia/Tokyo", startAt: "2026-10-12T10:00", endAt: "2026-10-12T12:00", projectId: "project-1", projectTitle: "Product research", colorIndex: 1, date: "2026-10-12", startMinute: 600, endMinute: 720, startLabel: "10:00", endLabel: "12:00", canUpdate: true, canDelete: true, canUpdateFollowing: true, canDeleteFollowing: true }],
  activeIntervals: dates.map((date) => ({ date, startMinute: 420, endMinute: 1380 })),
  pausedRules: [], allocatedMinutes: 600, targetMinutes: 1080, differenceMinutes: -480, freeMinutes: 4680,
  canEdit: true, busy: false,
  onChangeWeek: () => {}, onEditPlan: () => {}, onCreateAllocation: () => {}, onEditAllocation: () => {}, onEditActiveTime: () => {},
}

const meta = {
  title: "Weekly Plan/Board",
  component: WeeklyPlanBoard,
  args,
  decorators: [(Story: () => ReactNode) => <MemoryRouter><Story /></MemoryRouter>],
  parameters: { layout: "fullscreen" },
} satisfies Meta<typeof WeeklyPlanBoard>

export default meta
type Story = StoryObj<typeof meta>

export const Populated: Story = {}
