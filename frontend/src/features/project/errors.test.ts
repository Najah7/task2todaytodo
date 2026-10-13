import { describe, expect, it } from "vitest"
import { ApiError } from "~/api/http"
import { getProjectErrorMessageKey } from "./errors"

describe("getProjectErrorMessageKey", () => {
  it("explains the Free active Project cap", () => {
    const error = new ApiError(409, { error: { details: [{ code: "free_project_limit_reached" }] } })
    expect(getProjectErrorMessageKey(error)).toBe("projects.error.freeLimit")
  })
})
