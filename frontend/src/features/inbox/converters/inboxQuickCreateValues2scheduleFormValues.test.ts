import { describe, expect, it } from "vitest"
import { inboxQuickCreateValues2scheduleFormValues } from "~/features/inbox/converters/inboxQuickCreateValues2scheduleFormValues"
import { emptyInboxQuickCreateValues } from "~/features/inbox/components/InboxQuickCreate/schema"
import { scheduleFormValues2ruleCreateRequests } from "~/features/schedule/converters/scheduleFormValues2ruleCreateRequests"

describe("inboxQuickCreateValues2scheduleFormValues", () => {
  it("reuses schedule request conversion for one-off and inclusive all-day date ranges", () => {
    const [values] = inboxQuickCreateValues2scheduleFormValues([{
      ...emptyInboxQuickCreateValues,
      title: "Away",
      projectId: "project-1",
      startDate: "2026-10-12",
      endDate: "2026-10-14",
      allDay: true,
      timezone: "Asia/Tokyo",
    }], ["user-1"])
    const [request] = scheduleFormValues2ruleCreateRequests([values])

    expect(request).toMatchObject({
      title: "Away",
      project_id: "project-1",
      assignee_id: "user-1",
      frequency: "once",
      interval: 0,
      all_day: true,
      local_start: "2026-10-12T00:00:00",
      local_end: "2026-10-15T00:00:00",
      timezone: "Asia/Tokyo",
    })
    expect(request).not.toHaveProperty("end_date")
  })
})
