import { describe, expect, it } from "vitest"
import { ApiError } from "~/api/http"
import { allocationErrorMessage } from "./errorMessage"

const translate = (key: string, params?: Record<string, string | number>) => `${key}:${JSON.stringify(params ?? {})}`

describe("allocationErrorMessage", () => {
  it("formats overlap metadata with localized project and timezone details", () => {
    const error = new ApiError(409, {
      error: {
        details: [{ metadata: {
          kind: "outside_active_time",
          project_id: "project-1",
          start_at: "2026-10-12T01:00:00Z",
          end_at: "2026-10-12T02:00:00Z",
        } }],
      },
    })
    expect(allocationErrorMessage(error, translate, {
      status: 409,
      timezone: "Asia/Tokyo",
      language: "en",
      projectNames: new Map([["project-1", "Planning"]]),
    })).toBe('weeklyPlan.error.outsideActiveTime:{"project":"Planning","start":"Oct 12, 2026, 10:00","end":"Oct 12, 2026, 11:00"}')
  })

  it("distinguishes a stale revision from an overlap conflict", () => {
    expect(allocationErrorMessage(new ApiError(412, {}), translate, { status: 412 }))
      .toBe("weeklyPlan.error.revision:{}")
  })
})
