package repository

import (
	"context"

	"github.com/Najah7/task2todaytodo/internal/application/allocation/dao"
)

func (r *AllocationRepository) ListRuleRevisionsForProjection(ctx context.Context, userID string) ([]dao.Rule, error) {
	rows, err := r.queries.ListProjectAllocationRuleRevisionsForProjection(ctx, userID)
	if err != nil {
		return nil, err
	}
	rules := make([]dao.Rule, 0, len(rows))
	for _, row := range rows {
		pausedReason := ""
		if row.PauseReason.Valid {
			pausedReason = row.PauseReason.String
		}
		rule := dao.Rule{
			ID: row.RuleID, ProjectID: row.ProjectID, Revision: row.Revision,
			CurrentRevision:  projectionRevision(row.CurrentRuleRevision, row.Revision),
			SourceRevisionID: row.SourceRevisionID, Weekday: int(row.Weekday),
			StartTime: wallString(row.LocalStart), EndTime: wallString(row.LocalEnd),
			EndDayOffset: int(row.EndDayOffset), Timezone: row.Timezone,
			Frequency: row.Frequency, EffectiveFrom: dateValue(row.EffectiveFrom),
			RecurrenceStartDate: dateValue(row.RecurrenceStartDate),
			EffectiveAt:         row.EffectiveAt.Time, PausedReason: pausedReason,
		}
		if row.EffectiveUntil.Valid {
			value := dateValue(row.EffectiveUntil)
			rule.EffectiveUntil = &value
		}
		if row.EffectiveUntilAt.Valid {
			value := row.EffectiveUntilAt.Time
			rule.EffectiveUntilAt = &value
		}
		if row.PausedAt.Valid {
			value := row.PausedAt.Time
			rule.PausedAt = &value
		}
		if row.StoppedAfter.Valid {
			value := dateValue(row.StoppedAfter)
			rule.StoppedAfter = &value
		}
		if row.DeletedAt.Valid {
			value := row.DeletedAt.Time
			rule.DeletedAt = &value
		}
		rules = append(rules, rule)
	}
	return rules, nil
}

func projectionRevision(value any, fallback int32) int32 {
	switch revision := value.(type) {
	case int32:
		return revision
	case int64:
		return int32(revision)
	case int:
		return int32(revision)
	default:
		return fallback
	}
}

var _ interface {
	ListRuleRevisionsForProjection(context.Context, string) ([]dao.Rule, error)
} = (*AllocationRepository)(nil)
