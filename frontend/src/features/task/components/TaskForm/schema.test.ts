import { describe, expect, it } from "vitest"
import { durationTextToMinutes, minutesToDurationText, taskCreateSchema } from "./schema"

describe("Task form estimate values", () => {
  it("converts between the displayed h:mm value and API minutes", () => {
    expect(durationTextToMinutes("0:45")).toBe(45)
    expect(durationTextToMinutes("2:05")).toBe(125)
    expect(durationTextToMinutes("30")).toBe(30)
    expect(durationTextToMinutes("1:99")).toBeUndefined()
    expect(durationTextToMinutes("2147483647")).toBe(2_147_483_647)
    expect(durationTextToMinutes("2147483648")).toBeUndefined()
    expect(minutesToDurationText(125)).toBe("2:05")
    expect(minutesToDurationText(Number.POSITIVE_INFINITY)).toBe("")
    expect(minutesToDurationText(undefined)).toBe("")
  })

  it("requires a positive manual estimate for childless Tasks and derives it for Tasks with ActionItems", () => {
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
      manualEstimate: "",
      actionItems: [],
    }).success).toBe(false)
    expect(taskCreateSchema.safeParse({
      title: "Prepare report",
      projectId: "",
      dueDate: "",
      description: "",
      priority: "low",
      manualEstimate: "0",
      actionItems: [],
    }).success).toBe(false)
    expect(taskCreateSchema.safeParse({
      title: "Prepare report",
      projectId: "",
      dueDate: "",
      description: "",
      priority: "low",
      manualEstimate: "0:15",
      actionItems: [],
    }).success).toBe(true)
    expect(taskCreateSchema.safeParse({
      title: "Prepare report",
      projectId: "",
      dueDate: "",
      description: "",
      priority: "low",
      manualEstimate: "",
      actionItems: [{ title: "Draft", estimatedMinutes: "0:30", priority: "", pendingDelete: true }],
    }).success).toBe(false)
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
      actionItems: [{ title: "Draft", estimatedMinutes: "0", priority: "" }],
    }).success).toBe(false)
    expect(taskCreateSchema.safeParse({
      title: "Prepare report",
      projectId: "",
      dueDate: "",
      description: "",
      priority: "",
      manualEstimate: "",
      actionItems: [{ title: "Draft", estimatedMinutes: "1", priority: "" }],
    }).success).toBe(true)
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

  it("allows whole nonnegative actual minutes only on completed ActionItems", () => {
    const base = {
      title: "Prepare report",
      projectId: "",
      dueDate: "",
      description: "",
      priority: "",
      manualEstimate: "",
    }
    expect(taskCreateSchema.safeParse({ ...base, actionItems: [{ title: "Draft", estimatedMinutes: "30", priority: "", completed: true, actualMinutes: "0" }] }).success).toBe(true)
    expect(taskCreateSchema.safeParse({ ...base, actionItems: [{ title: "Draft", estimatedMinutes: "30", priority: "", completed: true, actualMinutes: "-1" }] }).success).toBe(false)
    expect(taskCreateSchema.safeParse({ ...base, actionItems: [{ title: "Draft", estimatedMinutes: "30", priority: "", completed: true, actualMinutes: "1.5" }] }).success).toBe(false)
    expect(taskCreateSchema.safeParse({ ...base, actionItems: [{ title: "Draft", estimatedMinutes: "30", priority: "", completed: true, actualMinutes: "2147483648" }] }).success).toBe(false)
  })
})
