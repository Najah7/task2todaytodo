import { describe, expect, it } from "vitest"
import { ApiError } from "~/api/http"
import { taskErrorMessageKey } from "./errors"

describe("taskErrorMessageKey", () => {
  it("explains Project and ActionItem Free caps", () => {
    expect(taskErrorMessageKey(new ApiError(409, { error: { details: [{ code: "free_project_limit_reached" }] } })))
      .toBe("tasks.form.freeProjectLimit")
    expect(taskErrorMessageKey(new ApiError(409, { error: { details: [{ code: "free_action_item_limit_reached" }] } })))
      .toBe("tasks.form.freeActionItemLimit")
  })
})
