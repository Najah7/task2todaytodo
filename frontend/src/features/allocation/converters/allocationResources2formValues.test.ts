import { expect, test } from "vitest"
import { allocationResources2formValues } from "./allocationResources2formValues"

test("merges rule-only and plan-only projects while preserving explicit zero targets", () => {
  const [values] = allocationResources2formValues(
    [{ revision: "rules-v1", timezone: "Asia/Tokyo", items: [
      { id: "rule-1", revision: 1, project_id: "rules-only", project_title: "Rules only", weekday: 1, start_time: "09:00", end_time: "10:00", end_day_offset: 0, timezone: "Asia/Tokyo" },
    ] }],
    [{ revision: "plans-v1", items: [
      { project_id: "plan-only", title: "Plan only", target_minutes: 0 },
    ] }],
  )

  expect(values?.projects).toEqual([
    { projectId: "plan-only", targetMinutes: 0, hasTarget: true, rules: [] },
    { projectId: "rules-only", targetMinutes: 0, hasTarget: false, rules: [expect.objectContaining({ id: "rule-1", revision: 1, weekday: "1" })] },
  ])
})
