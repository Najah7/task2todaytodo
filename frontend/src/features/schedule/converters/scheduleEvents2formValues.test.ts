import { describe, expect, it } from "vitest"
import { scheduleBlueprintRevisions2formValues } from "~/features/schedule/converters/scheduleBlueprintRevisions2formValues"
import { scheduleEvents2formValues } from "~/features/schedule/converters/scheduleEvents2formValues"
import type { ScheduleBlueprintRevision, ScheduleEvent } from "~/features/schedule/providers/ScheduleAPIDataProvider/context"

const blueprint: ScheduleBlueprintRevision = {
  revisionId: "source-1", revision: 4, effectiveFrom: "2026-01-01", superseded: false, deleted: false,
  title: "Daily", description: "", location: "", projectId: "", assigneeId: "user", timezone: "UTC",
  allDay: false, localStart: "2026-01-01T09:00:00", localEnd: "2026-01-02T10:00:00", frequency: "daily",
  interval: 1, weekdays: [], endDate: "2026-10-15",
}

describe("Schedule data to form values", () => {
  it("keeps following recurrence anchored to the original date", () => {
    const values = scheduleBlueprintRevisions2formValues([blueprint], "2026-10-10")[0]
    expect(values.startDate).toBe("2026-10-10")
    expect(values.endDate).toBe("2026-10-11")
    expect(values.recurrenceAnchorDate).toBe("2026-10-10")
    expect(values.recurrenceEndDate).toBe("2026-10-15")
    expect(values.scope).toBe("following")
  })

  it("keeps a moved current occurrence's displayed date while retaining its original recurrence anchor", () => {
    const event: ScheduleEvent = {
      id: "schedule-1", blueprintId: "blueprint-1", occurrenceDate: "2026-10-10", title: "Moved", description: "",
      location: "", start: new Date("2026-10-20T09:00:00Z"), end: new Date("2026-10-20T10:00:00Z"),
      localStart: "2026-10-20T09:00:00", localEnd: "2026-10-20T10:00:00", startDate: "2026-10-20", endDate: "2026-10-20",
      timezone: "UTC", allDay: false, projectId: "", projectTitle: "", assigneeId: "user", sourceRevisionId: "source-1",
      revision: 2, persisted: true, state: "scheduled", permissions: {
        canUpdate: true, canDelete: true, canComplete: true, canSkip: true, canRestore: false, canAssign: true,
        canUpdateFollowing: true, canDeleteFollowing: true,
      },
    }
    const values = scheduleEvents2formValues([event], [blueprint])[0]
    expect(values.startDate).toBe("2026-10-20")
    expect(values.recurrenceAnchorDate).toBe("2026-10-10")
    expect(values.recurrenceEndDate).toBe("2026-10-15")
  })
})
