import type { ScheduleRuleRevision } from "~/features/schedule/providers/ScheduleAPIDataProvider/context"

export function selectEffectiveRuleRevision(
  revisions: ScheduleRuleRevision[],
  occurrenceDate: string,
  sourceRevisionId?: string,
): ScheduleRuleRevision | undefined {
  const source = sourceRevisionId ? revisions.find((revision) => revision.revisionId === sourceRevisionId) : undefined
  if (source && !source.superseded && !source.deleted && coversDate(source, occurrenceDate)) return source

  return revisions
    .filter((revision) => !revision.superseded && !revision.deleted && coversDate(revision, occurrenceDate))
    .sort((left, right) => right.effectiveFrom.localeCompare(left.effectiveFrom) || right.revision - left.revision)[0]
}

function coversDate(revision: ScheduleRuleRevision, occurrenceDate: string): boolean {
  return revision.effectiveFrom <= occurrenceDate && (!revision.effectiveUntil || occurrenceDate <= revision.effectiveUntil)
}
