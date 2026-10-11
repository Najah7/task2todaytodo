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

var _ usecase.BlueprintRepository = (*ScheduleRepository)(nil)

func (r *ScheduleRepository) GetBlueprint(ctx context.Context, actorID, blueprintID string) (dao.Blueprint, error) {
	row, err := r.queries.GetScheduleBlueprintByActor(ctx, sqlc.GetScheduleBlueprintByActorParams{BlueprintID: blueprintID, ActorID: actorID})
	if errors.Is(err, pgx.ErrNoRows) {
		return dao.Blueprint{}, domain.ErrScheduleNotFound
	}
	if err != nil {
		return dao.Blueprint{}, err
	}
	blueprint := blueprintFromValues(row.ID, row.UserID, boolValue(row.Deleted), unix(row.CreatedAt), row.Revision,
		row.EffectiveFrom, row.RecurrenceAnchorDate, row.LocalStart, row.LocalEnd, row.Timezone, row.Frequency,
		row.IntervalCount, row.Weekdays, row.EndDate, row.Title, row.Description, row.Location, row.ProjectID,
		row.AssigneeID)
	blueprint.CutoffAfter = nullableDateValue(row.CutoffAfter)
	return blueprint, nil
}

func (r *ScheduleRepository) GetEffectiveRevisionForDate(ctx context.Context, actorID, blueprintID, occurrenceDate string, capability shared.Capability) (dao.BlueprintRevision, error) {
	row, err := r.queries.GetEffectiveScheduleBlueprintRevisionForDate(ctx, sqlc.GetEffectiveScheduleBlueprintRevisionForDateParams{
		BlueprintID: blueprintID, OccurrenceDate: pgDate(parseDate(occurrenceDate)), ActorID: actorID,
		ResourceID: string(capability.Resource), Action: sqlc.Action(capability.Action),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return dao.BlueprintRevision{}, usecase.ErrOccurrenceNotFound
	}
	if err != nil {
		return dao.BlueprintRevision{}, err
	}
	definition := blueprintFromValues(row.BlueprintID, row.UserID, false, unix(row.CreatedAt), row.Revision,
		row.EffectiveFrom, row.RecurrenceAnchorDate, row.LocalStart, row.LocalEnd, row.Timezone, row.Frequency,
		row.IntervalCount, row.Weekdays, row.EndDate, row.Title, row.Description, row.Location, row.ProjectID,
		row.AssigneeID)
	definition.CutoffAfter = nullableDateValue(row.CutoffAfter)
	return dao.BlueprintRevision{Blueprint: definition, RevisionID: row.ID,
		EffectiveUntil: nullableDateValue(row.EffectiveUntil),
		ChangedBy:      row.ChangedBy, ChangedAt: unix(row.CreatedAt)}, nil
}

func (r *ScheduleRepository) GetBlueprintRevisionForMutation(ctx context.Context, actorID, blueprintID, occurrenceDate string, capability shared.Capability) (int32, error) {
	revision, err := r.queries.GetScheduleBlueprintRevisionForMutation(ctx, sqlc.GetScheduleBlueprintRevisionForMutationParams{
		BlueprintID: blueprintID, OccurrenceDate: pgDate(parseDate(occurrenceDate)), ActorID: actorID,
		ResourceID: string(capability.Resource), Action: sqlc.Action(capability.Action),
	})
	if err != nil {
		return 0, err
	}
	return revision, nil
}

func (r *ScheduleRepository) CreateBlueprint(ctx context.Context, actorID string, definition dao.BlueprintRevision, first *dao.ScheduleOccurrence) (dao.Blueprint, error) {
	project := pgText(definition.ProjectID)
	blueprint, err := r.queries.CreateScheduleBlueprintByActor(ctx, sqlc.CreateScheduleBlueprintByActorParams{
		BlueprintID: definition.ID, ProjectID: project, ActorID: actorID, AssigneeID: definition.AssigneeID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return dao.Blueprint{}, usecase.ErrPermissionDenied
		}
		return dao.Blueprint{}, err
	}
	if definition.AssigneeID == "" {
		definition.AssigneeID = blueprint.UserID
	}
	revision, err := r.queries.CreateScheduleBlueprintRevisionByActor(ctx, revisionParams(definition, actorID))
	if err != nil {
		return dao.Blueprint{}, err
	}
	if first != nil {
		if _, err := r.queries.CreateScheduleOccurrence(ctx, sqlc.CreateScheduleOccurrenceParams{
			ID: derefID(first.ID), OccurrenceDate: pgDate(parseDate(first.OccurrenceDate)),
			StartAt: pgTime(time.Unix(first.StartAt, 0).UTC()), EndAt: pgTime(time.Unix(first.EndAt, 0).UTC()),
			ActorID: actorID, SourceRevisionID: revision.ID,
		}); err != nil {
			return dao.Blueprint{}, err
		}
	}
	return blueprintFromRevision(revision, false, unix(blueprint.CreatedAt)), nil
}

func (r *ScheduleRepository) ListBlueprintRevisions(ctx context.Context, actorID, blueprintID string, limit int, before int32) ([]dao.BlueprintRevision, error) {
	var cursor pgtype.Int4
	if before > 0 {
		cursor = pgtype.Int4{Int32: before, Valid: true}
	}
	rows, err := r.queries.ListScheduleBlueprintRevisionsByActor(ctx, sqlc.ListScheduleBlueprintRevisionsByActorParams{
		BlueprintID: blueprintID, ActorID: actorID, BeforeRevision: cursor, PageLimit: int32(limit),
	})
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 && before == 0 {
		if _, getErr := r.GetBlueprint(ctx, actorID, blueprintID); getErr != nil {
			return nil, getErr
		}
	}
	out := make([]dao.BlueprintRevision, 0, len(rows))
	for _, row := range rows {
		definition := blueprintFromValues(row.BlueprintID, row.UserID, boolValue(row.BlueprintDeleted), 0, row.Revision,
			row.EffectiveFrom, row.RecurrenceAnchorDate, row.LocalStart, row.LocalEnd, row.Timezone, row.Frequency,
			row.IntervalCount, row.Weekdays, row.EndDate, row.Title, row.Description, row.Location, row.ProjectID,
			row.AssigneeID)
		definition.CutoffAfter = nullableDateValue(row.CutoffAfter)
		out = append(out, dao.BlueprintRevision{Blueprint: definition, RevisionID: row.ID, Superseded: row.SupersededAt.Valid,
			ChangedBy: row.ChangedBy, ChangedAt: unix(row.CreatedAt)})
	}
	return out, nil
}

func (r *ScheduleRepository) ListOccurrenceRevisions(ctx context.Context, actorID, blueprintID, occurrenceDate string, limit int, before int32) ([]dao.ScheduleOccurrenceRevision, error) {
	var cursor pgtype.Int4
	if before > 0 {
		cursor = pgtype.Int4{Int32: before, Valid: true}
	}
	rows, err := r.queries.ListScheduleOccurrenceRevisionsByActor(ctx, sqlc.ListScheduleOccurrenceRevisionsByActorParams{
		BlueprintID: blueprintID, OccurrenceDate: pgDate(parseDate(occurrenceDate)), ActorID: actorID,
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
			OccurrenceDate: dateString(row.OccurrenceDate), StartAt: unix(row.StartAt), EndAt: unix(row.EndAt),
			Title: row.Title, Description: nullableText(row.Description), Location: nullableText(row.Location),
			ProjectID: nullableText(row.ProjectID), AssigneeID: row.AssigneeID, Timezone: row.Timezone,
			Completed: row.Completed, SkippedAt: skippedAt, DeletedAt: deletedAt,
			IsOverride: row.IsOverride, ChangedBy: row.ChangedBy, ChangedAt: unix(row.ChangedAt),
		})
	}
	return out, nil
}

func (r *ScheduleRepository) ListBlueprintRevisionsForRange(ctx context.Context, actorID string, from, to time.Time) ([]dao.BlueprintRevision, error) {
	rows, err := r.queries.ListScheduleBlueprintRevisionsForRangeByActor(ctx, sqlc.ListScheduleBlueprintRevisionsForRangeByActorParams{
		FromAt: pgTime(from), ToAt: pgTime(to), ActorID: actorID,
	})
	if err != nil {
		return nil, err
	}
	out := make([]dao.BlueprintRevision, 0, len(rows))
	for _, row := range rows {
		definition := blueprintFromValues(row.BlueprintID, row.UserID, false, 0, row.Revision,
			row.EffectiveFrom, row.RecurrenceAnchorDate, row.LocalStart, row.LocalEnd, row.Timezone, row.Frequency,
			row.IntervalCount, row.Weekdays, row.EndDate, row.Title, row.Description, row.Location, row.ProjectID,
			row.AssigneeID)
		definition.CutoffAfter = nullableDateValue(row.CutoffAfter)
		effectiveUntil := nullableDateValue(row.EffectiveUntil)
		out = append(out, dao.BlueprintRevision{Blueprint: definition, RevisionID: row.ID, EffectiveUntil: effectiveUntil,
			ChangedBy: row.ChangedBy, ChangedAt: unix(row.CreatedAt)})
	}
	return out, nil
}

func (r *ScheduleRepository) ListPersistedOccurrences(ctx context.Context, actorID string, from, to time.Time) ([]dao.ScheduleOccurrence, error) {
	rows, err := r.queries.ListScheduleOccurrencesForRangeByActor(ctx, sqlc.ListScheduleOccurrencesForRangeByActorParams{FromAt: pgTime(from), ToAt: pgTime(to), ActorID: actorID})
	if err != nil {
		return nil, err
	}
	out := make([]dao.ScheduleOccurrence, 0, len(rows))
	for _, row := range rows {
		out = append(out, occurrenceFromRow(row))
	}
	return out, nil
}

func (r *ScheduleRepository) ListOccurrenceOrigins(ctx context.Context, _ string, blueprintIDs []string, from, to time.Time) ([]dao.ScheduleOccurrenceOrigin, error) {
	if len(blueprintIDs) == 0 {
		return []dao.ScheduleOccurrenceOrigin{}, nil
	}
	rows, err := r.queries.ListScheduleOccurrenceOriginsForRangeByActor(ctx, sqlc.ListScheduleOccurrenceOriginsForRangeByActorParams{FromAt: pgTime(from), ToAt: pgTime(to), BlueprintIds: blueprintIDs})
	if err != nil {
		return nil, err
	}
	out := make([]dao.ScheduleOccurrenceOrigin, 0, len(rows))
	for _, row := range rows {
		out = append(out, dao.ScheduleOccurrenceOrigin{BlueprintID: row.BlueprintID, OccurrenceDate: dateString(row.OccurrenceDate)})
	}
	return out, nil
}

func (r *ScheduleRepository) ListPauseIntervals(ctx context.Context, _, blueprintID string) ([]dao.ProjectPauseInterval, error) {
	rows, err := r.queries.ListSchedulePauseIntervalsByActor(ctx, blueprintID)
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

func (r *ScheduleRepository) PauseProjectBlueprints(ctx context.Context, actorID, projectID string, at time.Time) error {
	return r.queries.PauseScheduleBlueprintsForProject(ctx, sqlc.PauseScheduleBlueprintsForProjectParams{ProjectID: projectID, PausedAt: pgTime(at), ActorID: actorID})
}

func (r *ScheduleRepository) ResumeProjectBlueprints(ctx context.Context, actorID, projectID string, at time.Time) error {
	return r.queries.ResumeScheduleBlueprintsForProject(ctx, sqlc.ResumeScheduleBlueprintsForProjectParams{ResumedAt: pgTime(at), ActorID: actorID, ProjectID: projectID})
}

func (r *ScheduleRepository) GetPersistedOccurrence(ctx context.Context, actorID, blueprintID, occurrenceDate string) (dao.ScheduleOccurrence, error) {
	row, err := r.queries.GetScheduleOccurrenceByActor(ctx, sqlc.GetScheduleOccurrenceByActorParams{
		BlueprintID: blueprintID, OccurrenceDate: pgDate(parseDate(occurrenceDate)), ActorID: actorID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return dao.ScheduleOccurrence{}, usecase.ErrOccurrenceNotFound
	}
	if err != nil {
		return dao.ScheduleOccurrence{}, err
	}
	return occurrenceFromRow(row), nil
}

func (r *ScheduleRepository) GetPersistedOccurrenceForHistory(ctx context.Context, actorID, blueprintID, occurrenceDate string) (dao.ScheduleOccurrence, error) {
	row, err := r.queries.GetScheduleOccurrenceForHistoryByActor(ctx, sqlc.GetScheduleOccurrenceForHistoryByActorParams{
		BlueprintID: blueprintID, OccurrenceDate: pgDate(parseDate(occurrenceDate)), ActorID: actorID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return dao.ScheduleOccurrence{}, usecase.ErrOccurrenceNotFound
	}
	if err != nil {
		return dao.ScheduleOccurrence{}, err
	}
	return occurrenceFromRow(row), nil
}

func (r *ScheduleRepository) GetPersistedOccurrenceForMutation(ctx context.Context, actorID, blueprintID, occurrenceDate string, capability shared.Capability) (dao.ScheduleOccurrence, error) {
	row, err := r.queries.GetScheduleOccurrenceForMutation(ctx, sqlc.GetScheduleOccurrenceForMutationParams{
		BlueprintID: blueprintID, OccurrenceDate: pgDate(parseDate(occurrenceDate)), ActorID: actorID,
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

func (r *ScheduleRepository) LockOccurrenceForMutation(ctx context.Context, actorID, blueprintID, occurrenceDate string, capability shared.Capability) error {
	_, err := r.queries.LockScheduleOccurrenceForMutation(ctx, sqlc.LockScheduleOccurrenceForMutationParams{
		BlueprintID: blueprintID, OccurrenceDate: pgDate(parseDate(occurrenceDate)), ActorID: actorID,
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
		Title: occurrence.Title, Description: pgText(occurrence.Description), Location: pgText(occurrence.Location),
		ProjectID: pgText(occurrence.ProjectID), AssigneeID: occurrence.AssigneeID, Timezone: occurrence.Timezone,
		Completed: occurrence.Completed, SkippedAt: optionalUnix(occurrence.Skipped, occurrence.UpdatedAt),
		DeletedAt: optionalUnix(occurrence.Deleted, occurrence.UpdatedAt), IsOverride: occurrence.IsOverride,
		ActorID: actorID, SourceRevisionID: occurrence.SourceRevisionID, BlueprintID: occurrence.BlueprintID,
		ResourceID: string(capability.Resource), Action: sqlc.Action(capability.Action),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		if _, readErr := r.GetPersistedOccurrenceForMutation(ctx, actorID, occurrence.BlueprintID, occurrence.OccurrenceDate, capability); readErr == nil {
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
		EndAt: pgTime(time.Unix(occurrence.EndAt, 0).UTC()), Title: occurrence.Title,
		Description: pgText(occurrence.Description), Location: pgText(occurrence.Location), ProjectID: pgText(occurrence.ProjectID),
		AssigneeID: occurrence.AssigneeID, Timezone: occurrence.Timezone, Completed: occurrence.Completed,
		SkippedAt: optionalUnix(occurrence.Skipped, occurrence.UpdatedAt), DeletedAt: optionalUnix(occurrence.Deleted, occurrence.UpdatedAt),
		IsOverride: occurrence.IsOverride, ActorID: actorID, BlueprintID: occurrence.BlueprintID,
		OccurrenceDate: pgDate(parseDate(occurrence.OccurrenceDate)), ExpectedRevision: expectedRevision,
		ResourceID: string(capability.Resource), Action: sqlc.Action(capability.Action),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		if _, readErr := r.GetPersistedOccurrenceForMutation(ctx, actorID, occurrence.BlueprintID, occurrence.OccurrenceDate, capability); readErr == nil {
			return dao.ScheduleOccurrence{}, usecase.ErrOccurrenceRevisionMismatch
		}
		return dao.ScheduleOccurrence{}, usecase.ErrPermissionDenied
	}
	if err != nil {
		return dao.ScheduleOccurrence{}, err
	}
	return occurrenceFromRow(row), nil
}

func (r *ScheduleRepository) PersistOccurrence(ctx context.Context, actorID string, occurrence dao.ScheduleOccurrence, expected *int32, capability shared.Capability) (dao.ScheduleOccurrence, error) {
	var revision int32
	if expected != nil {
		revision = *expected
	}
	row, err := r.queries.UpsertScheduleOccurrenceByActor(ctx, sqlc.UpsertScheduleOccurrenceByActorParams{
		ID: derefID(occurrence.ID), BlueprintID: occurrence.BlueprintID, SourceRevisionID: occurrence.SourceRevisionID,
		UserID: occurrence.UserID, OccurrenceDate: pgDate(parseDate(occurrence.OccurrenceDate)),
		StartAt: pgTime(time.Unix(occurrence.StartAt, 0).UTC()), EndAt: pgTime(time.Unix(occurrence.EndAt, 0).UTC()),
		Title: occurrence.Title, Description: pgText(occurrence.Description), Location: pgText(occurrence.Location),
		ProjectID: pgText(occurrence.ProjectID), AssigneeID: occurrence.AssigneeID, Timezone: occurrence.Timezone,
		Completed: occurrence.Completed, SkippedAt: optionalUnix(occurrence.Skipped, occurrence.StartAt),
		DeletedAt: optionalUnix(occurrence.Deleted, occurrence.UpdatedAt), IsOverride: occurrence.IsOverride,
		ActorID: actorID, ResourceID: string(capability.Resource), Action: sqlc.Action(capability.Action), ExpectedRevision: revision,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return dao.ScheduleOccurrence{}, usecase.ErrPermissionDenied
	}
	if err != nil {
		return dao.ScheduleOccurrence{}, err
	}
	return occurrenceFromRow(row), nil
}

func (r *ScheduleRepository) PublishRevision(ctx context.Context, actorID, blueprintID string, revision dao.BlueprintRevision, expectedRevision int32, capability shared.Capability) (dao.Blueprint, error) {
	params := publishRevisionParams(revision, actorID, blueprintID)
	params.ExpectedRevision = expectedRevision
	params.ResourceID = string(capability.Resource)
	params.Action = sqlc.Action(capability.Action)
	row, err := r.queries.PublishScheduleBlueprintRevisionByActor(ctx, params)
	if errors.Is(err, pgx.ErrNoRows) {
		return dao.Blueprint{}, usecase.ErrOccurrenceRevisionMismatch
	}
	if err != nil {
		return dao.Blueprint{}, err
	}
	return blueprintFromRevision(row, false, 0), nil
}

func (r *ScheduleRepository) DeleteFollowing(ctx context.Context, actorID, blueprintID, occurrenceDate, revisionID string, expectedRevision int32) error {
	rows, err := r.queries.DeleteScheduleBlueprintFollowingByActor(ctx, sqlc.DeleteScheduleBlueprintFollowingByActorParams{
		BlueprintID: blueprintID, ActorID: actorID, OccurrenceDate: pgDate(parseDate(occurrenceDate)), RevisionID: revisionID, ExpectedRevision: expectedRevision,
	})
	if err == nil && rows == 0 {
		return usecase.ErrOccurrenceRevisionMismatch
	}
	return err
}

func (r *ScheduleRepository) ListOccurrenceProjectIDsForFollowing(ctx context.Context, blueprintID, occurrenceDate string) ([]string, error) {
	rows, err := r.queries.ListScheduleOccurrenceProjectIDsForFollowing(ctx, sqlc.ListScheduleOccurrenceProjectIDsForFollowingParams{
		BlueprintID: blueprintID, OccurrenceDate: pgDate(parseDate(occurrenceDate)),
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

func (r *ScheduleRepository) LockBlueprintForRevisionMutation(ctx context.Context, actorID, blueprintID, occurrenceDate string, capability shared.Capability) error {
	_, err := r.queries.LockScheduleBlueprintForRevisionMutation(ctx, sqlc.LockScheduleBlueprintForRevisionMutationParams{
		BlueprintID: blueprintID, OccurrenceDate: pgDate(parseDate(occurrenceDate)), ActorID: actorID,
		ResourceID: string(capability.Resource), Action: sqlc.Action(capability.Action),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return usecase.ErrOccurrenceRevisionMismatch
	}
	return err
}

func (r *ScheduleRepository) ListBlueprintEligibleAssignees(ctx context.Context, actorID, blueprintID string) ([]dao.Assignee, error) {
	rows, err := r.queries.ListEligibleScheduleBlueprintAssignees(ctx, sqlc.ListEligibleScheduleBlueprintAssigneesParams{BlueprintID: blueprintID, ActorID: actorID})
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
	return r.queries.IsScheduleBlueprintAssigneeEligible(ctx, sqlc.IsScheduleBlueprintAssigneeEligibleParams{
		ProjectID: pgText(projectID), OwnerID: ownerID, AssigneeID: assigneeID,
	})
}

func (r *ScheduleRepository) ListRevisionsForBackfill(ctx context.Context) ([]dao.BlueprintRevision, error) {
	rows, err := r.queries.ListScheduleBlueprintRevisionsForBackfill(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]dao.BlueprintRevision, 0, len(rows))
	for _, row := range rows {
		out = append(out, backfillRevisionFromValues(row.ID, row.BlueprintID, row.UserID, row.Revision,
			row.EffectiveFrom, row.RecurrenceAnchorDate, row.LocalStart, row.LocalEnd, row.Timezone, row.Frequency,
			row.IntervalCount, row.Weekdays, row.EndDate, row.Title, row.Description, row.Location, row.ProjectID,
			row.AssigneeID, row.ChangedBy, row.CreatedAt, row.EffectiveUntil))
	}
	return out, nil
}

func (r *ScheduleRepository) ListRevisionsForBlueprintBackfill(ctx context.Context, blueprintID string) ([]dao.BlueprintRevision, error) {
	rows, err := r.queries.ListScheduleBlueprintRevisionsForBackfillByBlueprint(ctx, blueprintID)
	if err != nil {
		return nil, err
	}
	out := make([]dao.BlueprintRevision, 0, len(rows))
	for _, row := range rows {
		out = append(out, backfillRevisionFromValues(row.ID, row.BlueprintID, row.UserID, row.Revision,
			row.EffectiveFrom, row.RecurrenceAnchorDate, row.LocalStart, row.LocalEnd, row.Timezone, row.Frequency,
			row.IntervalCount, row.Weekdays, row.EndDate, row.Title, row.Description, row.Location, row.ProjectID,
			row.AssigneeID, row.ChangedBy, row.CreatedAt, row.EffectiveUntil))
	}
	return out, nil
}

func (r *ScheduleRepository) LockBlueprintForBackfill(ctx context.Context, blueprintID string) error {
	_, err := r.queries.LockScheduleBlueprintForBackfill(ctx, blueprintID)
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

func revisionParams(revision dao.BlueprintRevision, actorID string) sqlc.CreateScheduleBlueprintRevisionByActorParams {
	definition := revision.Blueprint
	return sqlc.CreateScheduleBlueprintRevisionByActorParams{
		RevisionID: revision.RevisionID, EffectiveFrom: pgDate(parseDate(definition.EffectiveFrom)),
		RecurrenceAnchorDate: pgDate(parseDate(definition.RecurrenceAnchorDate)), LocalStart: pgLocal(definition.LocalStart),
		LocalEnd: pgLocal(definition.LocalEnd), Timezone: definition.Timezone, Frequency: definition.Frequency,
		IntervalCount: int32(definition.Interval), Weekdays: append([]string{}, definition.Weekdays...), EndDate: optionalDate(definition.EndDate),
		Title: definition.Title, Description: pgText(definition.Description), Location: pgText(definition.Location),
		ProjectID: pgText(definition.ProjectID), AssigneeID: definition.AssigneeID, ActorID: actorID, BlueprintID: definition.ID,
	}
}

func publishRevisionParams(revision dao.BlueprintRevision, actorID, blueprintID string) sqlc.PublishScheduleBlueprintRevisionByActorParams {
	definition := revision.Blueprint
	return sqlc.PublishScheduleBlueprintRevisionByActorParams{
		RevisionID: revision.RevisionID, EffectiveFrom: pgDate(parseDate(definition.EffectiveFrom)),
		RecurrenceAnchorDate: pgDate(parseDate(definition.RecurrenceAnchorDate)), LocalStart: pgLocal(definition.LocalStart),
		LocalEnd: pgLocal(definition.LocalEnd), Timezone: definition.Timezone, Frequency: definition.Frequency,
		IntervalCount: int32(definition.Interval), Weekdays: append([]string{}, definition.Weekdays...), EndDate: optionalDate(definition.EndDate),
		Title: definition.Title, Description: pgText(definition.Description), Location: pgText(definition.Location),
		ProjectID: pgText(definition.ProjectID), AssigneeID: definition.AssigneeID, ActorID: actorID, BlueprintID: blueprintID,
		CutoffAfter: optionalDate(definition.CutoffAfter),
	}
}

func blueprintFromRevision(row sqlc.ScheduleBlueprintRevision, deleted bool, createdAt int64) dao.Blueprint {
	blueprint := blueprintFromValues(row.BlueprintID, row.UserID, deleted, createdAt, row.Revision,
		row.EffectiveFrom, row.RecurrenceAnchorDate, row.LocalStart, row.LocalEnd, row.Timezone, row.Frequency,
		row.IntervalCount, row.Weekdays, row.EndDate, row.Title, row.Description, row.Location, row.ProjectID,
		row.AssigneeID)
	blueprint.CutoffAfter = nullableDateValue(row.CutoffAfter)
	return blueprint
}

func backfillRevisionFromValues(id, blueprintID, userID string, revision int32, effective, anchor pgtype.Date,
	localStart, localEnd pgtype.Timestamp, timezone, frequency string, interval int32, weekdays []string,
	endDate pgtype.Date, title string, description, location, project pgtype.Text, assignee, changedBy string,
	createdAt pgtype.Timestamptz, effectiveUntil interface{}) dao.BlueprintRevision {
	definition := blueprintFromValues(blueprintID, userID, false, 0, revision, effective, anchor, localStart, localEnd,
		timezone, frequency, interval, weekdays, endDate, title, description, location, project, assignee)
	return dao.BlueprintRevision{Blueprint: definition, RevisionID: id, EffectiveUntil: nullableDateValue(effectiveUntil),
		ChangedBy: changedBy, ChangedAt: unix(createdAt)}
}

func blueprintFromValues(id, userID string, deleted bool, createdAt int64, revision int32,
	effective, anchor pgtype.Date, localStart, localEnd pgtype.Timestamp, timezone, frequency string,
	interval int32, weekdays []string, endDate pgtype.Date, title string, description, location, project pgtype.Text,
	assignee string) dao.Blueprint {
	var end *string
	if endDate.Valid {
		value := dateString(endDate)
		end = &value
	}
	return dao.Blueprint{ID: id, UserID: userID, Deleted: deleted, CreatedAt: createdAt, Revision: revision,
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
	return dao.ScheduleOccurrence{ID: &id, BlueprintID: row.BlueprintID, OccurrenceDate: dateString(row.OccurrenceDate),
		SourceRevisionID: row.SourceRevisionID, UserID: row.UserID, ProjectID: nullableText(row.ProjectID),
		AssigneeID: row.AssigneeID, Title: row.Title, Description: nullableText(row.Description), Location: nullableText(row.Location),
		Timezone: row.Timezone, StartAt: unix(row.StartAt), EndAt: unix(row.EndAt), Completed: row.Completed,
		Skipped: skipped, Deleted: deleted, IsOverride: row.IsOverride, Revision: row.Revision,
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
