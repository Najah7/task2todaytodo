import { describe, expect, it } from "vitest"
import { projectMembers2assigneeOptions } from "~/features/schedule/converters/projectMembers2assigneeOptions"

describe("projectMembers2assigneeOptions", () => {
  it("keeps owner and current-user choices when project members cannot be listed", () => {
    const options = projectMembers2assigneeOptions(
      [],
      undefined,
      { user_id: "editor-1", user_name: "Editor" },
      "Project owner",
      "owner-1",
      "Current user",
    )

    expect(options).toEqual([
      { id: "editor-1", displayName: "Editor" },
      { id: "owner-1", displayName: "Project owner" },
    ])
  })
})
