package repository

import (
	"context"
	"errors"
	"time"

	"github.com/Najah7/task2todaytodo/db/sqlc"
	"github.com/Najah7/task2todaytodo/internal/application/schedule/dao"
	"github.com/Najah7/task2todaytodo/internal/application/schedule/domain"
	"github.com/Najah7/task2todaytodo/internal/application/schedule/usecase"
	"github.com/Najah7/task2todaytodo/internal/application/shared"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

var _ usecase.RuleRepository = (*ScheduleRepository)(nil)

func (r *ScheduleRepository) GetRule(ctx context.Context, actorID, ruleID string) (dao.Rule, error) {
	row, err := r.queries.GetScheduleRuleByActor(ctx, sqlc.GetScheduleRuleByActorParams{RuleID: ruleID, ActorID: actorID})
	if errors.Is(err, pgx.ErrNoRows) {
		return dao.Rule{}, domain.ErrScheduleNotFound
	}
	if err != nil {
		return dao.Rule{}, err
	}
	rule := ruleFromValues(row.ID, row.UserID, boolValue(row.Deleted), unix(row.CreatedAt), row.Revision,
		row.EffectiveFrom, row.RecurrenceAnchorDate, row.LocalStart, row.LocalEnd, row.Timezone, row.Frequency,
		row.IntervalCount, row.Weekdays, row.EndDate, row.Title, row.Description, row.Location, row.ProjectID,
		row.AssigneeID)
	rule.CutoffAfter = nullableDateValue(row.CutoffAfter)
	rule.AllDay = row.AllDay
	return rule, nil
}

func (r *ScheduleRepository) GetEffectiveRevisionForDate(ctx context.Context, actorID, ruleID, occurrenceDate string, capability shared.Capability) (dao.RuleRevision, error) {
	row, err := r.queries.GetEffectiveScheduleRuleRevisionForDate(ctx, sqlc.GetEffectiveScheduleRuleRevisionForDateParams{
		RuleID: ruleID, OccurrenceDate: pgDate(parseDate(occurrenceDate)), ActorID: actorID,
		ResourceID: string(capability.Resource), Action: sqlc.Action(capability.Action),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return dao.RuleRevision{}, usecase.ErrOccurrenceNotFound
	}
	if err != nil {
		return dao.RuleRevision{}, err
	}
	definition := ruleFromValues(row.RuleID, row.UserID, false, unix(row.CreatedAt), row.Revision,
		row.EffectiveFrom, row.RecurrenceAnchorDate, row.LocalStart, row.LocalEnd, row.Timezone, row.Frequency,
		row.IntervalCount, row.Weekdays, row.EndDate, row.Title, row.Description, row.Location, row.ProjectID,
		row.AssigneeID)
	definition.CutoffAfter = nullableDateValue(row.CutoffAfter)
	definition.AllDay = row.AllDay
	return dao.RuleRevision{Rule: definition, RevisionID: row.ID,
		EffectiveUntil: nullableDateValue(row.EffectiveUntil),
		ChangedBy:      row.ChangedBy, ChangedAt: unix(row.CreatedAt)}, nil
}

func (r *ScheduleRepository) GetRuleRevisionForMutation(ctx context.Context, actorID, ruleID, occurrenceDate string, capability shared.Capability) (int32, error) {
	revision, err := r.queries.GetScheduleRuleRevisionForMutation(ctx, sqlc.GetScheduleRuleRevisionForMutationParams{
		RuleID: ruleID, OccurrenceDate: pgDate(parseDate(occurrenceDate)), ActorID: actorID,
		ResourceID: string(capability.Resource), Action: sqlc.Action(capability.Action),
	})
	if err != nil {
		return 0, err
	}
	return revision, nil
}

func (r *ScheduleRepository) CreateRule(ctx context.Context, actorID string, definition dao.RuleRevision, first *dao.ScheduleOccurrence) (dao.Rule, error) {
	project := pgText(definition.ProjectID)
	rule, err := r.queries.CreateScheduleRuleByActor(ctx, sqlc.CreateScheduleRuleByActorParams{
		RuleID: definition.ID, ProjectID: project, ActorID: actorID, AssigneeID: definition.AssigneeID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return dao.Rule{}, usecase.ErrPermissionDenied
		}
		return dao.Rule{}, err
	}
	if definition.AssigneeID == "" {
		definition.AssigneeID = rule.UserID
	}
	revision, err := r.queries.CreateScheduleRuleRevisionByActor(ctx, revisionParams(definition, actorID))
	if err != nil {
		return dao.Rule{}, err
	}
	if first != nil {
		if _, err := r.queries.CreateScheduleOccurrence(ctx, sqlc.CreateScheduleOccurrenceParams{
			ID: derefID(first.ID), OccurrenceDate: pgDate(parseDate(first.OccurrenceDate)),
			StartAt: pgTime(time.Unix(first.StartAt, 0).UTC()), EndAt: pgTime(time.Unix(first.EndAt, 0).UTC()),
			ActorID: actorID, SourceRevisionID: revision.ID,
		}); err != nil {
			return dao.Rule{}, err
		}
	}
	return ruleFromRevision(revision, false, unix(rule.CreatedAt)), nil
}

func (r *ScheduleRepository) ListRuleRevisions(ctx context.Context, actorID, ruleID string, limit int, before int32) ([]dao.RuleRevision, error) {
	var cursor pgtype.Int4
	if before > 0 {
		cursor = pgtype.Int4{Int32: before, Valid: true}
	}
	rows, err := r.queries.ListScheduleRuleRevisionsByActor(ctx, sqlc.ListScheduleRuleRevisionsByActorParams{
		RuleID: ruleID, ActorID: actorID, BeforeRevision: cursor, PageLimit: int32(limit),
	})
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 && before == 0 {
		if _, getErr := r.GetRule(ctx, actorID, ruleID); getErr != nil {
			return nil, getErr
		}
	}
	out := make([]dao.RuleRevision, 0, len(rows))
	for _, row := range rows {
		definition := ruleFromValues(row.RuleID, row.UserID, boolValue(row.RuleDeleted), 0, row.Revision,
			row.EffectiveFrom, row.RecurrenceAnchorDate, row.LocalStart, row.LocalEnd, row.Timezone, row.Frequency,
			row.IntervalCount, row.Weekdays, row.EndDate, row.Title, row.Description, row.Location, row.ProjectID,
			row.AssigneeID)
		definition.CutoffAfter = nullableDateValue(row.CutoffAfter)
		definition.AllDay = row.AllDay
		out = append(out, dao.RuleRevision{Rule: definition, RevisionID: row.ID, Superseded: row.SupersededAt.Valid,
			EffectiveUntil: nullableDateValue(row.EffectiveUntil), ChangedBy: row.ChangedBy, ChangedAt: unix(row.CreatedAt)})
	}
	return out, nil
}

func (r *ScheduleRepository) ListOccurrenceRevisions(ctx context.Context, actorID, ruleID, occurrenceDate string, limit int, before int32) ([]dao.ScheduleOccurrenceRevision, error) {
	var cursor pgtype.Int4
	if before > 0 {
		cursor = pgtype.Int4{Int32: before, Valid: true}
	}
	rows, err := r.queries.ListScheduleOccurrenceRevisionsByActor(ctx, sqlc.ListScheduleOccurrenceRevisionsByActorParams{
		RuleID: ruleID, OccurrenceDate: pgDate(parseDate(occurrenceDate)), ActorID: actorID,
		CursorRevision: cursor, PageLimit: int32(limit),
	})
	if err != nil {
		return nil, err
	}
	out := make([]dao.ScheduleOccurrenceRevision, 0, len(rows))
	for _, row := range rows {
		var skippedAt, deletedAt *int64
		if row.SkippedAt.Valid {
			value := row.SkippedAt.Time.Unix()
			skippedAt = &value
		}
		if row.DeletedAt.Valid {
			value := row.DeletedAt.Time.Unix()
			deletedAt = &value
		}
		out = append(out, dao.ScheduleOccurrenceRevision{
			Revision: row.Revision, SourceRevisionID: row.SourceRevisionID, UserID: row.UserID,
			OccurrenceDate: dateString(row.OccurrenceDate), StartAt: unix(row.StartAt), EndAt: unix(row.EndAt), AllDay: row.AllDay,
			Title: row.Title, Description: nullableText(row.Description), Location: nullableText(row.Location),
			ProjectID: nullableText(row.ProjectID), AssigneeID: row.AssigneeID, Timezone: row.Timezone,
			Completed: row.Completed, SkippedAt: skippedAt, DeletedAt: deletedAt,
			ChangedBy: row.ChangedBy, ChangedAt: unix(row.ChangedAt),
		})
	}
	return out, nil
}

func (r *ScheduleRepository) ListRuleRevisionsForRange(ctx context.Context, actorID string, from, to time.Time) ([]dao.RuleRevision, error) {
	rows, err := r.queries.ListScheduleRuleRevisionsForRangeByActor(ctx, sqlc.ListScheduleRuleRevisionsForRangeByActorParams{
		FromAt: pgTime(from), ToAt: pgTime(to), ActorID: actorID,
	})
	if err != nil {
		return nil, err
	}
	out := make([]dao.RuleRevision, 0, len(rows))
	for _, row := range rows {
		definition := ruleFromValues(row.RuleID, row.UserID, false, 0, row.Revision,
			row.EffectiveFrom, row.RecurrenceAnchorDate, row.LocalStart, row.LocalEnd, row.Timezone, row.Frequency,
			row.IntervalCount, row.Weekdays, row.EndDate, row.Title, row.Description, row.Location, row.ProjectID,
			row.AssigneeID)
		definition.CutoffAfter = nullableDateValue(row.CutoffAfter)
		definition.AllDay = row.AllDay
		effectiveUntil := nullableDateValue(row.EffectiveUntil)
		out = append(out, dao.RuleRevision{Rule: definition, RevisionID: row.ID, EffectiveUntil: effectiveUntil,
			ChangedBy: row.ChangedBy, ChangedAt: unix(row.CreatedAt)})
	}
	return out, nil
}

func (r *ScheduleRepository) ListPersistedOccurrences(ctx context.Context, actorID string, from, to time.Time, includeSkipped bool) ([]dao.ScheduleOccurrence, error) {
	rows, err := r.queries.ListScheduleOccurrencesForRangeByActor(ctx, sqlc.ListScheduleOccurrencesForRangeByActorParams{FromAt: pgTime(from), ToAt: pgTime(to), ActorID: actorID, IncludeSkipped: includeSkipped})
	if err != nil {
		return nil, err
	}
	out := make([]dao.ScheduleOccurrence, 0, len(rows))
	for _, row := range rows {
		out = append(out, occurrenceFromRow(row))
	}
	return out, nil
}

func (r *ScheduleRepository) ListOccurrenceCapabilities(ctx context.Context, actorID string, occurrences []dao.ScheduleOccurrence) (map[string]dao.OccurrenceCapabilities, error) {
	if len(occurrences) == 0 {
		return map[string]dao.OccurrenceCapabilities{}, nil
	}
	ruleIDs := make([]string, len(occurrences))
	occurrenceDates := make([]string, len(occurrences))
	for i, occurrence := range occurrences {
		ruleIDs[i] = occurrence.RuleID
		occurrenceDates[i] = occurrence.OccurrenceDate
	}
	rows, err := r.queries.ListScheduleOccurrenceCapabilities(ctx, sqlc.ListScheduleOccurrenceCapabilitiesParams{
		ActorID: actorID, RuleIds: ruleIDs, OccurrenceDates: occurrenceDates,
	})
	if err != nil {
		return nil, err
	}
	result := make(map[string]dao.OccurrenceCapabilities, len(rows))
	for _, row := range rows {
		result[row.RuleID+"/"+dateString(row.OccurrenceDate)] = dao.OccurrenceCapabilities{
			CanUpdate: row.CanUpdate, CanDelete: row.CanDelete, CanSkip: row.CanSkip, CanAssign: row.CanAssign,
			CanUpdateFollowing: row.CanUpdateFollowing, CanDeleteFollowing: row.CanDeleteFollowing,
		}
	}
	return result, nil
}

func (r *ScheduleRepository) ListOccurrenceOrigins(ctx context.Context, _ string, ruleIDs []string, from, to time.Time) ([]dao.ScheduleOccurrenceOrigin, error) {
	if len(ruleIDs) == 0 {
		return []dao.ScheduleOccurrenceOrigin{}, nil
	}
	rows, err := r.queries.ListScheduleOccurrenceOriginsForRangeByActor(ctx, sqlc.ListScheduleOccurrenceOriginsForRangeByActorParams{FromAt: pgTime(from), ToAt: pgTime(to), RuleIds: ruleIDs})
	if err != nil {
		return nil, err
	}
	out := make([]dao.ScheduleOccurrenceOrigin, 0, len(rows))
	for _, row := range rows {
		out = append(out, dao.ScheduleOccurrenceOrigin{RuleID: row.RuleID, OccurrenceDate: dateString(row.OccurrenceDate)})
	}
	return out, nil
}

func (r *ScheduleRepository) ListPauseIntervals(ctx context.Context, _, ruleID string) ([]dao.ProjectPauseInterval, error) {
	rows, err := r.queries.ListSchedulePauseIntervalsByActor(ctx, ruleID)
	if err != nil {
		return nil, err
	}
	out := make([]dao.ProjectPauseInterval, 0, len(rows))
	for _, row := range rows {
		var resumed *time.Time
		if row.ResumedAt.Valid {
			value := row.ResumedAt.Time
			resumed = &value
		}
		out = append(out, dao.ProjectPauseInterval{ProjectID: row.ProjectID, PausedAt: row.PausedAt.Time, ResumedAt: resumed})
	}
	return out, nil
}

func (r *ScheduleRepository) PauseProjectRules(ctx context.Context, actorID, projectID string, at time.Time) error {
	return r.queries.PauseScheduleRulesForProject(ctx, sqlc.PauseScheduleRulesForProjectParams{ProjectID: projectID, PausedAt: pgTime(at), ActorID: actorID})
}

func (r *ScheduleRepository) ResumeProjectRules(ctx context.Context, actorID, projectID string, at time.Time) error {
	return r.queries.ResumeScheduleRulesForProject(ctx, sqlc.ResumeScheduleRulesForProjectParams{ResumedAt: pgTime(at), ActorID: actorID, ProjectID: projectID})
}

func (r *ScheduleRepository) GetPersistedOccurrence(ctx context.Context, actorID, ruleID, occurrenceDate string) (dao.ScheduleOccurrence, error) {
	row, err := r.queries.GetScheduleOccurrenceByActor(ctx, sqlc.GetScheduleOccurrenceByActorParams{
		RuleID: ruleID, OccurrenceDate: pgDate(parseDate(occurrenceDate)), ActorID: actorID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return dao.ScheduleOccurrence{}, usecase.ErrOccurrenceNotFound
	}
	if err != nil {
		return dao.ScheduleOccurrence{}, err
	}
	return occurrenceFromRow(row), nil
}

func (r *ScheduleRepository) GetPersistedOccurrenceForHistory(ctx context.Context, actorID, ruleID, occurrenceDate string) (dao.ScheduleOccurrence, error) {
	row, err := r.queries.GetScheduleOccurrenceForHistoryByActor(ctx, sqlc.GetScheduleOccurrenceForHistoryByActorParams{
		RuleID: ruleID, OccurrenceDate: pgDate(parseDate(occurrenceDate)), ActorID: actorID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return dao.ScheduleOccurrence{}, usecase.ErrOccurrenceNotFound
	}
	if err != nil {
		return dao.ScheduleOccurrence{}, err
	}
	return occurrenceFromRow(row), nil
}

func (r *ScheduleRepository) GetPersistedOccurrenceForMutation(ctx context.Context, actorID, ruleID, occurrenceDate string, capability shared.Capability) (dao.ScheduleOccurrence, error) {
	row, err := r.queries.GetScheduleOccurrenceForMutation(ctx, sqlc.GetScheduleOccurrenceForMutationParams{
		RuleID: ruleID, OccurrenceDate: pgDate(parseDate(occurrenceDate)), ActorID: actorID,
		ResourceID: string(capability.Resource), Action: sqlc.Action(capability.Action),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return dao.ScheduleOccurrence{}, usecase.ErrOccurrenceNotFound
	}
	if err != nil {
		return dao.ScheduleOccurrence{}, err
	}
	return occurrenceFromRow(row), nil
}

func (r *ScheduleRepository) LockOccurrenceForMutation(ctx context.Context, actorID, ruleID, occurrenceDate string, capability shared.Capability) error {
	_, err := r.queries.LockScheduleOccurrenceForMutation(ctx, sqlc.LockScheduleOccurrenceForMutationParams{
		RuleID: ruleID, OccurrenceDate: pgDate(parseDate(occurrenceDate)), ActorID: actorID,
		ResourceID: string(capability.Resource), Action: sqlc.Action(capability.Action),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return usecase.ErrPermissionDenied
	}
	return err
}

func (r *ScheduleRepository) MaterializeOccurrence(ctx context.Context, actorID string, occurrence dao.ScheduleOccurrence, capability shared.Capability) (dao.ScheduleOccurrence, error) {
	row, err := r.queries.MaterializeScheduleOccurrenceByActor(ctx, sqlc.MaterializeScheduleOccurrenceByActorParams{
		ID: derefID(occurrence.ID), OccurrenceDate: pgDate(parseDate(occurrence.OccurrenceDate)),
		StartAt: pgTime(time.Unix(occurrence.StartAt, 0).UTC()), EndAt: pgTime(time.Unix(occurrence.EndAt, 0).UTC()),
		AllDay: occurrence.AllDay, Title: occurrence.Title, Description: pgText(occurrence.Description), Location: pgText(occurrence.Location),
		ProjectID: pgText(occurrence.ProjectID), AssigneeID: occurrence.AssigneeID, Timezone: occurrence.Timezone,
		Completed: occurrence.Completed, SkippedAt: optionalUnix(occurrence.Skipped, occurrence.UpdatedAt),
		DeletedAt: optionalUnix(occurrence.Deleted, occurrence.UpdatedAt),
		ActorID:   actorID, SourceRevisionID: occurrence.SourceRevisionID, RuleID: occurrence.RuleID,
		ResourceID: string(capability.Resource), Action: sqlc.Action(capability.Action),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		if _, readErr := r.GetPersistedOccurrenceForMutation(ctx, actorID, occurrence.RuleID, occurrence.OccurrenceDate, capability); readErr == nil {
			return dao.ScheduleOccurrence{}, usecase.ErrOccurrenceRevisionMismatch
		}
		return dao.ScheduleOccurrence{}, usecase.ErrPermissionDenied
	}
	if err != nil {
		return dao.ScheduleOccurrence{}, err
	}
	return occurrenceFromRow(row), nil
}

func (r *ScheduleRepository) UpdatePersistedOccurrence(ctx context.Context, actorID string, occurrence dao.ScheduleOccurrence, expectedRevision int32, capability shared.Capability) (dao.ScheduleOccurrence, error) {
	row, err := r.queries.UpdatePersistedScheduleOccurrenceByActor(ctx, sqlc.UpdatePersistedScheduleOccurrenceByActorParams{
		SourceRevisionID: occurrence.SourceRevisionID, StartAt: pgTime(time.Unix(occurrence.StartAt, 0).UTC()),
		EndAt: pgTime(time.Unix(occurrence.EndAt, 0).UTC()), AllDay: occurrence.AllDay, Title: occurrence.Title,
		Description: pgText(occurrence.Description), Location: pgText(occurrence.Location), ProjectID: pgText(occurrence.ProjectID),
		AssigneeID: occurrence.AssigneeID, Timezone: occurrence.Timezone, Completed: occurrence.Completed,
		SkippedAt: optionalUnix(occurrence.Skipped, occurrence.UpdatedAt), DeletedAt: optionalUnix(occurrence.Deleted, occurrence.UpdatedAt),
		ActorID: actorID, RuleID: occurrence.RuleID,
		OccurrenceDate: pgDate(parseDate(occurrence.OccurrenceDate)), ExpectedRevision: expectedRevision,
		ResourceID: string(capability.Resource), Action: sqlc.Action(capability.Action),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		if _, readErr := r.GetPersistedOccurrenceForMutation(ctx, actorID, occurrence.RuleID, occurrence.OccurrenceDate, capability); readErr == nil {
			return dao.ScheduleOccurrence{}, usecase.ErrOccurrenceRevisionMismatch
		}
		return dao.ScheduleOccurrence{}, usecase.ErrPermissionDenied
	}
	if err != nil {
		return dao.ScheduleOccurrence{}, err
	}
	return occurrenceFromRow(row), nil
}

func (r *ScheduleRepository) ApplyRuleRevisionToOccurrence(ctx context.Context, actorID string, occurrence dao.ScheduleOccurrence) error {
	_, err := r.queries.ApplyScheduleRuleRevisionToOccurrence(ctx, sqlc.ApplyScheduleRuleRevisionToOccurrenceParams{
		SourceRevisionID: occurrence.SourceRevisionID, StartAt: pgTime(time.Unix(occurrence.StartAt, 0).UTC()),
		EndAt: pgTime(time.Unix(occurrence.EndAt, 0).UTC()), AllDay: occurrence.AllDay, Title: occurrence.Title,
		Description: pgText(occurrence.Description), Location: pgText(occurrence.Location), ProjectID: pgText(occurrence.ProjectID),
		AssigneeID: occurrence.AssigneeID, Timezone: occurrence.Timezone, ActorID: actorID,
		RuleID: occurrence.RuleID, OccurrenceDate: pgDate(parseDate(occurrence.OccurrenceDate)),
	})
	return err
}

func (r *ScheduleRepository) PublishRevision(ctx context.Context, actorID, ruleID string, revision dao.RuleRevision, expectedRevision int32, capability shared.Capability) (dao.Rule, error) {
	params := publishRevisionParams(revision, actorID, ruleID)
	params.ExpectedRevision = expectedRevision
	params.ResourceID = string(capability.Resource)
	params.Action = sqlc.Action(capability.Action)
	row, err := r.queries.PublishScheduleRuleRevisionByActor(ctx, params)
	if errors.Is(err, pgx.ErrNoRows) {
		return dao.Rule{}, usecase.ErrOccurrenceRevisionMismatch
	}
	if err != nil {
		return dao.Rule{}, err
	}
	return ruleFromRevision(row, false, 0), nil
}

func (r *ScheduleRepository) DeleteFollowing(ctx context.Context, actorID, ruleID, occurrenceDate, revisionID string, expectedRevision int32) error {
	rows, err := r.queries.DeleteScheduleRuleFollowingByActor(ctx, sqlc.DeleteScheduleRuleFollowingByActorParams{
		RuleID: ruleID, ActorID: actorID, OccurrenceDate: pgDate(parseDate(occurrenceDate)), RevisionID: revisionID, ExpectedRevision: expectedRevision,
	})
	if err == nil && rows == 0 {
		return usecase.ErrOccurrenceRevisionMismatch
	}
	return err
}

func (r *ScheduleRepository) ListOccurrenceProjectIDsForFollowing(ctx context.Context, ruleID, occurrenceDate string) ([]string, error) {
	rows, err := r.queries.ListScheduleOccurrenceProjectIDsForFollowing(ctx, sqlc.ListScheduleOccurrenceProjectIDsForFollowingParams{
		RuleID: ruleID, OccurrenceDate: pgDate(parseDate(occurrenceDate)),
	})
	if err != nil {
		return nil, err
	}
	projects := make([]string, 0, len(rows))
	for _, row := range rows {
		if row.Valid {
			projects = append(projects, row.String)
		}
	}
	return projects, nil
}

func (r *ScheduleRepository) LockRuleForRevisionMutation(ctx context.Context, actorID, ruleID, occurrenceDate string, capability shared.Capability) error {
	_, err := r.queries.LockScheduleRuleForRevisionMutation(ctx, sqlc.LockScheduleRuleForRevisionMutationParams{
		RuleID: ruleID, OccurrenceDate: pgDate(parseDate(occurrenceDate)), ActorID: actorID,
		ResourceID: string(capability.Resource), Action: sqlc.Action(capability.Action),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return usecase.ErrOccurrenceRevisionMismatch
	}
	return err
}

func (r *ScheduleRepository) ListRuleEligibleAssignees(ctx context.Context, actorID, ruleID string) ([]dao.Assignee, error) {
	rows, err := r.queries.ListEligibleScheduleRuleAssignees(ctx, sqlc.ListEligibleScheduleRuleAssigneesParams{RuleID: ruleID, ActorID: actorID})
	if err != nil {
		return nil, err
	}
	out := make([]dao.Assignee, 0, len(rows))
	for _, row := range rows {
		out = append(out, dao.Assignee{ID: row.ID, FirstName: row.FirstName, LastName: row.LastName, Email: row.Email, IsOwner: row.IsOwner})
	}
	return out, nil
}

func (r *ScheduleRepository) IsAssigneeEligible(ctx context.Context, projectID, ownerID, assigneeID string) (bool, error) {
	return r.queries.IsScheduleRuleAssigneeEligible(ctx, sqlc.IsScheduleRuleAssigneeEligibleParams{
		ProjectID: pgText(projectID), OwnerID: ownerID, AssigneeID: assigneeID,
	})
}

func (r *ScheduleRepository) ListRevisionsForBackfill(ctx context.Context) ([]dao.RuleRevision, error) {
	rows, err := r.queries.ListScheduleRuleRevisionsForBackfill(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]dao.RuleRevision, 0, len(rows))
	for _, row := range rows {
		out = append(out, backfillRevisionFromValues(row.ID, row.RuleID, row.UserID, row.Revision,
			row.EffectiveFrom, row.RecurrenceAnchorDate, row.LocalStart, row.LocalEnd, row.Timezone, row.Frequency,
			row.IntervalCount, row.Weekdays, row.EndDate, row.AllDay, row.Title, row.Description, row.Location, row.ProjectID,
			row.AssigneeID, row.ChangedBy, row.CreatedAt, row.EffectiveUntil))
	}
	return out, nil
}

func (r *ScheduleRepository) ListRevisionsForRuleBackfill(ctx context.Context, ruleID string) ([]dao.RuleRevision, error) {
	rows, err := r.queries.ListScheduleRuleRevisionsForBackfillByRule(ctx, ruleID)
	if err != nil {
		return nil, err
	}
	out := make([]dao.RuleRevision, 0, len(rows))
	for _, row := range rows {
		out = append(out, backfillRevisionFromValues(row.ID, row.RuleID, row.UserID, row.Revision,
			row.EffectiveFrom, row.RecurrenceAnchorDate, row.LocalStart, row.LocalEnd, row.Timezone, row.Frequency,
			row.IntervalCount, row.Weekdays, row.EndDate, row.AllDay, row.Title, row.Description, row.Location, row.ProjectID,
			row.AssigneeID, row.ChangedBy, row.CreatedAt, row.EffectiveUntil))
	}
	return out, nil
}

func (r *ScheduleRepository) LockRuleForBackfill(ctx context.Context, ruleID string) error {
	_, err := r.queries.LockScheduleRuleForBackfill(ctx, ruleID)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrScheduleNotFound
	}
	return err
}

func (r *ScheduleRepository) GetRevisionBackfillCheckpoint(ctx context.Context, revisionID string) (*string, error) {
	value, err := r.queries.GetScheduleRevisionBackfillCheckpoint(ctx, revisionID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	date := dateString(value)
	return &date, nil
}

func (r *ScheduleRepository) AdvanceRevisionBackfillCheckpoint(ctx context.Context, revisionID, throughDate string) error {
	return r.queries.AdvanceScheduleRevisionBackfillCheckpoint(ctx, sqlc.AdvanceScheduleRevisionBackfillCheckpointParams{
		RevisionID: revisionID, ThroughDate: pgDate(parseDate(throughDate)),
	})
}

func (r *ScheduleRepository) BackfillOccurrence(ctx context.Context, actorID string, occurrence dao.ScheduleOccurrence) (bool, error) {
	rows, err := r.queries.BackfillScheduleOccurrence(ctx, sqlc.BackfillScheduleOccurrenceParams{
		ID: derefID(occurrence.ID), OccurrenceDate: pgDate(parseDate(occurrence.OccurrenceDate)),
		StartAt: pgTime(time.Unix(occurrence.StartAt, 0).UTC()), EndAt: pgTime(time.Unix(occurrence.EndAt, 0).UTC()),
		ActorID: actorID, SourceRevisionID: occurrence.SourceRevisionID,
	})
	return rows > 0, err
}

func revisionParams(revision dao.RuleRevision, actorID string) sqlc.CreateScheduleRuleRevisionByActorParams {
	definition := revision.Rule
	return sqlc.CreateScheduleRuleRevisionByActorParams{
		RevisionID: revision.RevisionID, EffectiveFrom: pgDate(parseDate(definition.EffectiveFrom)),
		RecurrenceAnchorDate: pgDate(parseDate(definition.RecurrenceAnchorDate)), LocalStart: pgLocal(definition.LocalStart),
		LocalEnd: pgLocal(definition.LocalEnd), AllDay: definition.AllDay, Timezone: definition.Timezone, Frequency: definition.Frequency,
		IntervalCount: int32(definition.Interval), Weekdays: append([]string{}, definition.Weekdays...), EndDate: optionalDate(definition.EndDate),
		Title: definition.Title, Description: pgText(definition.Description), Location: pgText(definition.Location),
		ProjectID: pgText(definition.ProjectID), AssigneeID: definition.AssigneeID, ActorID: actorID, RuleID: definition.ID,
	}
}

func publishRevisionParams(revision dao.RuleRevision, actorID, ruleID string) sqlc.PublishScheduleRuleRevisionByActorParams {
	definition := revision.Rule
	return sqlc.PublishScheduleRuleRevisionByActorParams{
		RevisionID: revision.RevisionID, EffectiveFrom: pgDate(parseDate(definition.EffectiveFrom)),
		RecurrenceAnchorDate: pgDate(parseDate(definition.RecurrenceAnchorDate)), LocalStart: pgLocal(definition.LocalStart),
		LocalEnd: pgLocal(definition.LocalEnd), AllDay: definition.AllDay, Timezone: definition.Timezone, Frequency: definition.Frequency,
		IntervalCount: int32(definition.Interval), Weekdays: append([]string{}, definition.Weekdays...), EndDate: optionalDate(definition.EndDate),
		Title: definition.Title, Description: pgText(definition.Description), Location: pgText(definition.Location),
		ProjectID: pgText(definition.ProjectID), AssigneeID: definition.AssigneeID, ActorID: actorID, RuleID: ruleID,
		CutoffAfter: optionalDate(definition.CutoffAfter),
	}
}

func ruleFromRevision(row sqlc.ScheduleRuleRevision, deleted bool, createdAt int64) dao.Rule {
	rule := ruleFromValues(row.RuleID, row.UserID, deleted, createdAt, row.Revision,
		row.EffectiveFrom, row.RecurrenceAnchorDate, row.LocalStart, row.LocalEnd, row.Timezone, row.Frequency,
		row.IntervalCount, row.Weekdays, row.EndDate, row.Title, row.Description, row.Location, row.ProjectID,
		row.AssigneeID)
	rule.CutoffAfter = nullableDateValue(row.CutoffAfter)
	rule.AllDay = row.AllDay
	return rule
}

func backfillRevisionFromValues(id, ruleID, userID string, revision int32, effective, anchor pgtype.Date,
	localStart, localEnd pgtype.Timestamp, timezone, frequency string, interval int32, weekdays []string,
	endDate pgtype.Date, allDay bool, title string, description, location, project pgtype.Text, assignee, changedBy string,
	createdAt pgtype.Timestamptz, effectiveUntil interface{}) dao.RuleRevision {
	definition := ruleFromValues(ruleID, userID, false, 0, revision, effective, anchor, localStart, localEnd,
		timezone, frequency, interval, weekdays, endDate, title, description, location, project, assignee)
	definition.AllDay = allDay
	return dao.RuleRevision{Rule: definition, RevisionID: id, EffectiveUntil: nullableDateValue(effectiveUntil),
		ChangedBy: changedBy, ChangedAt: unix(createdAt)}
}

func ruleFromValues(id, userID string, deleted bool, createdAt int64, revision int32,
	effective, anchor pgtype.Date, localStart, localEnd pgtype.Timestamp, timezone, frequency string,
	interval int32, weekdays []string, endDate pgtype.Date, title string, description, location, project pgtype.Text,
	assignee string) dao.Rule {
	var end *string
	if endDate.Valid {
		value := dateString(endDate)
		end = &value
	}
	return dao.Rule{ID: id, UserID: userID, Deleted: deleted, CreatedAt: createdAt, Revision: revision,
		EffectiveFrom: dateString(effective), RecurrenceAnchorDate: dateString(anchor), LocalStart: localString(localStart),
		LocalEnd: localString(localEnd), Timezone: timezone, Frequency: frequency, Interval: int(interval),
		Weekdays: append([]string(nil), weekdays...), EndDate: end, Title: title, Description: nullableText(description),
		Location: nullableText(location), ProjectID: nullableText(project), AssigneeID: assignee}
}

func occurrenceFromRow(row sqlc.Schedule) dao.ScheduleOccurrence {
	id := row.ID
	var skipped, deleted bool
	if row.SkippedAt.Valid {
		skipped = true
	}
	if row.DeletedAt.Valid {
		deleted = true
	}
	return dao.ScheduleOccurrence{ID: &id, RuleID: row.RuleID, OccurrenceDate: dateString(row.OccurrenceDate),
		SourceRevisionID: row.SourceRevisionID, UserID: row.UserID, ProjectID: nullableText(row.ProjectID),
		AssigneeID: row.AssigneeID, Title: row.Title, Description: nullableText(row.Description), Location: nullableText(row.Location),
		Timezone: row.Timezone, AllDay: row.AllDay, StartAt: unix(row.StartAt), EndAt: unix(row.EndAt), Completed: row.Completed,
		Skipped: skipped, Deleted: deleted, Revision: row.Revision,
		CreatedAt: unix(row.CreatedAt), UpdatedAt: unix(row.UpdatedAt)}
}

func pgLocal(value string) pgtype.Timestamp {
	parsed, err := time.Parse("2006-01-02 15:04:05.999999999", value)
	if err != nil {
		parsed, _ = time.Parse("2006-01-02T15:04:05.999999999", value)
	}
	if parsed.IsZero() {
		return pgtype.Timestamp{}
	}
	return pgtype.Timestamp{Time: parsed, Valid: true}
}

func localString(value pgtype.Timestamp) string {
	if !value.Valid {
		return ""
	}
	return value.Time.Format("2006-01-02 15:04:05.999999999")
}

func parseDate(value string) time.Time {
	parsed, _ := time.Parse("2006-01-02", value)
	return parsed
}

func optionalDate(value *string) pgtype.Date {
	if value == nil {
		return pgtype.Date{}
	}
	return pgDate(parseDate(*value))
}

func nullableDateValue(value interface{}) *string {
	if value == nil {
		return nil
	}
	switch date := value.(type) {
	case pgtype.Date:
		if date.Valid {
			text := dateString(date)
			return &text
		}
	case time.Time:
		text := date.Format("2006-01-02")
		return &text
	case string:
		return &date
	case []byte:
		text := string(date)
		return &text
	}
	return nil
}

func derefID(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func optionalUnix(present bool, value int64) pgtype.Timestamptz {
	if !present {
		return pgtype.Timestamptz{}
	}
	return pgTime(time.Unix(value, 0).UTC())
}
