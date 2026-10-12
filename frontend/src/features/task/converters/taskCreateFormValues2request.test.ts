import { describe, expect, it } from "vitest"
import { taskCreateFormValues2requests } from "./taskCreateFormValues2request"
import type { TaskCreateFormValues } from "~/features/task/components/TaskForm/schema"

const base: TaskCreateFormValues = {
  title: "Prepare report",
  projectId: "",
  dueDate: "",
  description: "",
  priority: "high",
  manualEstimate: "",
  actionItems: [],
}

describe("taskCreateFormValues2requests", () => {
  it("sends the required manual estimate for a childless Task", () => {
    expect(taskCreateFormValues2requests([{ ...base, manualEstimate: "0:45" }])).toEqual([{
      title: "Prepare report",
      description: "",
      priority: "high",
      manual_estimated_minutes: 45,
    }])
  })

  it("sends the ActionItem estimate sum as the Task's initial estimate", () => {
    expect(taskCreateFormValues2requests([{
      ...base,
      actionItems: [
        { title: "Draft", estimatedMinutes: "0:45", priority: "" },
        { title: "Review", estimatedMinutes: "0:30", priority: "" },
        { title: "Discarded", estimatedMinutes: "1:00", priority: "", pendingDelete: true },
      ],
    }])).toEqual([{
      title: "Prepare report",
      description: "",
      priority: "high",
      manual_estimated_minutes: 75,
    }])
  })
})
