import { describe, expect, it } from "vitest"
import { durationTextToMinutes, minutesToDurationText, taskCreateSchema } from "./schema"

describe("Task form estimate values", () => {
  it("converts between the displayed h:mm value and API minutes", () => {
    expect(durationTextToMinutes("0:45")).toBe(45)
    expect(durationTextToMinutes("2:05")).toBe(125)
    expect(durationTextToMinutes("30")).toBe(30)
    expect(durationTextToMinutes("1:99")).toBeUndefined()
    expect(minutesToDurationText(125)).toBe("2:05")
    expect(minutesToDurationText(Number.POSITIVE_INFINITY)).toBe("")
    expect(minutesToDurationText(undefined)).toBe("")
  })

  it("allows empty estimates and rejects malformed durations", () => {
    expect(taskCreateSchema.safeParse({
      title: "Prepare report",
      projectId: "",
      dueDate: "",
      description: "",
      priority: "low",
      manualEstimate: "",
      actionItems: [{ title: "Draft", estimatedMinutes: "0:30", priority: "" }],
    }).success).toBe(true)
    expect(taskCreateSchema.safeParse({
      title: "Prepare report",
      projectId: "",
      dueDate: "",
      description: "",
      priority: "low",
      manualEstimate: "0:75",
      actionItems: [],
    }).success).toBe(false)
    expect(taskCreateSchema.safeParse({
      title: "Prepare report",
      projectId: "",
      dueDate: "",
      description: "",
      priority: "",
      manualEstimate: "",
      actionItems: [{ title: " ", estimatedMinutes: "", priority: "" }],
    }).success).toBe(false)
  })
})
