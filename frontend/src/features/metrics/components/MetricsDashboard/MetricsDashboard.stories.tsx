import type { Meta, StoryObj } from "@storybook/react-vite"
import MetricsDashboard from "."

const date = (offset: number) => new Date(Date.UTC(2026, 9, 13 - offset)).toISOString().slice(0, 10)

const meta = {
  title: "Metrics/MetricsDashboard",
  component: MetricsDashboard,
  parameters: { layout: "fullscreen" },
} satisfies Meta<typeof MetricsDashboard>

export default meta
type Story = StoryObj<typeof meta>

export const SevenDays: Story = {
  args: {
    period: 7,
    plannedItems: 28,
    completedItems: 20,
    completionRatio: 20 / 28,
    overdueCount: 8,
    isPartialToday: true,
    days: [
      { date: date(6), plannedItems: 4, completedItems: 3, completionRatio: 0.75 },
      { date: date(5), plannedItems: 5, completedItems: 4, completionRatio: 0.8 },
      { date: date(4), plannedItems: 0, completedItems: 0, completionRatio: null },
      { date: date(3), plannedItems: 6, completedItems: 5, completionRatio: 0.83 },
      { date: date(2), plannedItems: 3, completedItems: 2, completionRatio: 0.67 },
      { date: date(1), plannedItems: 6, completedItems: 5, completionRatio: 0.83 },
      { date: date(0), plannedItems: 4, completedItems: 1, completionRatio: 0.25 },
    ],
    overdueGroups: [
      { id: "project-1", title: "Website refresh", count: 4 },
      { id: "project-2", title: "Product research", count: 2 },
      { id: "no-project", title: "No project", count: 2 },
    ],
    otherOverdueCount: 0,
    hasPlannedWork: true,
    onPeriodChange: () => undefined,
  },
}

export const ThirtyDays: Story = {
  args: {
    ...SevenDays.args,
    period: 30,
    days: Array.from({ length: 30 }, (_, index) => ({
      date: date(29 - index),
      plannedItems: index % 6 === 0 ? 0 : 2 + index % 5,
      completedItems: index % 6 === 0 ? 0 : 1 + index % 3,
      completionRatio: index % 6 === 0 ? null : (1 + index % 3) / (2 + index % 5),
    })),
  },
}

export const NoPlannedWork: Story = {
  args: {
    ...SevenDays.args,
    plannedItems: 0,
    completedItems: 0,
    completionRatio: null,
    days: Array.from({ length: 7 }, (_, index) => ({ date: date(6 - index), plannedItems: 0, completedItems: 0, completionRatio: null })),
    overdueCount: 0,
    overdueGroups: [],
    otherOverdueCount: 0,
    hasPlannedWork: false,
  },
}
