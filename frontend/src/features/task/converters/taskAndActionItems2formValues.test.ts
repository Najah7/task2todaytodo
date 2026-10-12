import { describe, expect, it } from "vitest"
import type { RestTaskResponse } from "~/api/generated/tasks"
import { taskAndActionItems2formValues } from "./taskAndActionItems2formValues"

describe("taskAndActionItems2formValues", () => {
  it("maps completed childless Task estimate and actual time into the form", () => {
    const task = {
      id: "task-1",
      title: "Direct work",
      status: "done",
      actual_minutes: 0,
      manual_estimated_minutes: 45,
      estimate_source: "manual",
      priority: "medium",
      description: "",
    } as RestTaskResponse

    expect(taskAndActionItems2formValues([{ task, actionItems: [] }])[0]).toMatchObject({
      taskCompleted: true,
      actualMinutes: "0",
      manualEstimate: "0:45",
      actionItems: [],
    })
  })
})
