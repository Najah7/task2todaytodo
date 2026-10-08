package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Najah7/task2todaytodo/db/sqlc"
	"github.com/Najah7/task2todaytodo/internal/application/schedule/dao"
	"github.com/Najah7/task2todaytodo/internal/application/schedule/domain"
	"github.com/Najah7/task2todaytodo/internal/application/schedule/usecase"
	"github.com/Najah7/task2todaytodo/internal/application/shared"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/oklog/ulid/v2"
)

var _ usecase.ScheduleRepository = (*ScheduleRepository)(nil)

type ScheduleRepository struct{ queries *sqlc.Queries }

func NewScheduleRepository(db sqlc.DBTX) *ScheduleRepository {
	return &ScheduleRepository{queries: sqlc.New(db)}
}
func (r *ScheduleRepository) WithTx(tx pgx.Tx) *ScheduleRepository {
	return &ScheduleRepository{queries: r.queries.WithTx(tx)}
}

func (r *ScheduleRepository) LockProjectForScheduleMutation(ctx context.Context, projectID domain.ProjectID) error {
	if projectID == "" {
		return nil
	}
	if _, err := r.queries.LockActiveProjectForScheduleMutation(ctx, string(projectID)); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return usecase.ErrScheduleProjectNotFound
		}
		return err
	}
	return nil
}

func (r *ScheduleRepository) CheckProjectPermission(ctx context.Context, actor domain.UserID, projectID domain.ProjectID, capability shared.Capability) (bool, error) {
	allowed, err := r.queries.HasProjectPermission(ctx, sqlc.HasProjectPermissionParams{
		ProjectID: string(projectID), ActorID: string(actor), ResourceID: string(capability.Resource), Action: sqlc.Action(capability.Action),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return false, usecase.ErrScheduleProjectNotFound
	}
	return allowed, err
}

func (r *ScheduleRepository) LockSeriesProjectForMutation(ctx context.Context, actor domain.UserID, series domain.ScheduleID, capability shared.Capability) error {
	arg := sqlc.GetScheduleProjectByUserIDForPermissionParams{SeriesID: string(series), ActorID: string(actor), ResourceID: string(capability.Resource), Action: sqlc.Action(capability.Action)}
	initialProjectID, err := r.queries.GetScheduleProjectByUserIDForPermission(ctx, arg)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrScheduleNotFound
		}
		return err
	}
	if initialProjectID.Valid {
		if err := r.LockProjectForScheduleMutation(ctx, domain.ProjectID(initialProjectID.String)); err != nil {
			return err
		}
	}
	lockedProjectID, err := r.queries.LockScheduleByUserIDForPermission(ctx, sqlc.LockScheduleByUserIDForPermissionParams{SeriesID: string(series), ActorID: string(actor), ResourceID: string(capability.Resource), Action: sqlc.Action(capability.Action)})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrScheduleNotFound
		}
		return err
	}
	if !sameNullableText(initialProjectID, lockedProjectID) {
		return usecase.ErrScheduleScopeInvalid
	}
	currentProjectID, err := r.queries.GetScheduleProjectByUserIDForPermission(ctx, arg)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrScheduleNotFound
		}
		return err
	}
	if !sameNullableText(lockedProjectID, currentProjectID) {
		return usecase.ErrScheduleScopeInvalid
	}
	return nil
}

func sameNullableText(left, right pgtype.Text) bool {
	return left.Valid == right.Valid && (!left.Valid || left.String == right.String)
}

func (r *ScheduleRepository) ListScheduleRevisionsByActor(ctx context.Context, actor domain.UserID, id domain.ScheduleID, limit int, anchor *usecase.CursorAnchor) ([]dao.ScheduleRevision, error) {
	arg := sqlc.ListScheduleRevisionsByActorParams{ID: string(id), ActorID: string(actor), PageLimit: int32(limit)}
	if anchor != nil {
		if anchor.Revision < 1 {
			return nil, usecase.ErrInvalidSchedulePage
		}
		arg.CursorRevision = pgtype.Int4{Int32: anchor.Revision, Valid: true}
	}
	rows, err := r.queries.ListScheduleRevisionsByActor(ctx, arg)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 && anchor == nil {
		return nil, domain.ErrScheduleNotFound
	}
	out := make([]dao.ScheduleRevision, 0, len(rows))
	for _, row := range rows {
		var deletedAt *int64
		if row.DeletedAt.Valid {
			value := row.DeletedAt.Time.Unix()
			deletedAt = &value
		}
		var frequencyAnchor *string
		if row.FrequencyAnchorDate.Valid {
			value := dateString(row.FrequencyAnchorDate)
			frequencyAnchor = &value
		}
		out = append(out, dao.ScheduleRevision{
			ID: row.ID, Revision: row.Revision, UserID: row.UserID, ProjectID: nullableText(row.ProjectID),
			AssigneeID: row.AssigneeID, Title: row.Title, Description: nullableText(row.Description),
			Location: nullableText(row.Location), StartAt: unix(row.StartAt), EndAt: unix(row.EndAt),
			SeriesID: row.SeriesID, OccurrenceDate: dateString(row.OccurrenceDate), Timezone: row.Timezone,
			IsException: row.IsException, Completed: row.Completed, DeletedAt: deletedAt,
			RepeatState: nullableText(row.RepeatState), FrequencyAnchorDate: frequencyAnchor,
			IntervalWeeks: int(row.IntervalWeeks), Frequencies: frequencyDAO(row.Frequencies),
			CreatedAt: unix(row.CreatedAt), UpdatedAt: unix(row.UpdatedAt), ChangedBy: row.ChangedBy,
			ChangedAt: unix(row.ChangedAt), CursorAt: formatTime(row.ChangedAt),
		})
	}
	return out, nil
}

func (r *ScheduleRepository) GetByUserIDWithPermission(ctx context.Context, actor domain.UserID, id domain.ScheduleID, cap shared.Capability) (dao.Schedule, error) {
	row, err := r.queries.GetScheduleByUserIDForPermission(ctx, sqlc.GetScheduleByUserIDForPermissionParams{ID: string(id), ActorID: string(actor), ResourceID: string(cap.Resource), Action: sqlc.Action(cap.Action)})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return dao.Schedule{}, domain.ErrScheduleNotFound
		}
		return dao.Schedule{}, err
	}
	return scheduleRow(row), nil
}
func (r *ScheduleRepository) ListByAssigneeUserID(ctx context.Context, actor domain.UserID) ([]dao.Schedule, error) {
	rows, err := r.queries.ListSchedulesForOccurrenceProjectionByAssigneeUserID(ctx, string(actor))
	if err != nil {
		return nil, err
	}
	out := make([]dao.Schedule, 0, len(rows))
	for _, row := range rows {
		out = append(out, scheduleRow(row))
	}
	return out, nil
}
func (r *ScheduleRepository) ListByProjectAndUserID(ctx context.Context, actor domain.UserID, project domain.ProjectID) ([]dao.Schedule, error) {
	rows, err := r.queries.ListSchedulesForOccurrenceProjectionByProjectAndUserID(ctx, sqlc.ListSchedulesForOccurrenceProjectionByProjectAndUserIDParams{ProjectID: string(project), UserID: string(actor)})
	if err != nil {
		return nil, err
	}
	out := make([]dao.Schedule, 0, len(rows))
	for _, row := range rows {
		out = append(out, scheduleRow(row))
	}
	return out, nil
}
func (r *ScheduleRepository) ListOccurrencesForPermission(ctx context.Context, actor domain.UserID, series domain.ScheduleID, cap shared.Capability) ([]dao.Schedule, error) {
	rows, err := r.queries.ListSchedulesForOccurrenceCommandByUserID(ctx, sqlc.ListSchedulesForOccurrenceCommandByUserIDParams{SeriesID: string(series), ActorID: string(actor), ResourceID: string(cap.Resource), Action: sqlc.Action(cap.Action)})
	if err != nil {
		return nil, err
	}
	out := make([]dao.Schedule, 0, len(rows))
	for _, row := range rows {
		out = append(out, scheduleRow(row))
	}
	return out, nil
}
func (r *ScheduleRepository) ListActiveSeriesByAssigneeUserID(ctx context.Context, user domain.UserID) ([]dao.Schedule, error) {
	rows, err := r.queries.ListActiveScheduleSeriesByAssigneeUserID(ctx, string(user))
	if err != nil {
		return nil, err
	}
	out := make([]dao.Schedule, 0, len(rows))
	for _, row := range rows {
		out = append(out, scheduleRow(row))
	}
	return out, nil
}
func (r *ScheduleRepository) CreateByUserID(ctx context.Context, actor domain.UserID, s domain.Schedule) (dao.Schedule, error) {
	row, err := r.queries.CreateScheduleByUserID(ctx, sqlc.CreateScheduleByUserIDParams{ProjectID: pgText(string(s.ProjectID)), ActorID: string(actor), ID: string(s.ID), Title: s.Title, Description: pgText(s.Description), Location: pgText(s.Location), SeriesID: string(s.SeriesID), OccurrenceDate: pgDate(s.OccurrenceDate), Timezone: s.Timezone, IsException: s.IsException, StartAt: pgTime(s.StartAt), EndAt: pgTime(s.EndAt), IntervalWeeks: int32(s.IntervalWeeks), Frequencies: frequencyValues(s.Frequencies)})
	if err != nil {
		return dao.Schedule{}, err
	}
	return scheduleRow(row), nil
}
func (r *ScheduleRepository) UpdateByUserID(ctx context.Context, actor domain.UserID, s domain.Schedule, cap shared.Capability) (dao.Schedule, error) {
	row, err := r.queries.UpdateScheduleByUserID(ctx, sqlc.UpdateScheduleByUserIDParams{Title: s.Title, Description: pgText(s.Description), Location: pgText(s.Location), StartAt: pgTime(s.StartAt), EndAt: pgTime(s.EndAt), ID: string(s.ID), ActorID: string(actor)})
	if err != nil {
		return dao.Schedule{}, err
	}
	return scheduleRow(row), nil
}
func (r *ScheduleRepository) SetCompletedByUserID(ctx context.Context, actor domain.UserID, id domain.ScheduleID, completed bool, cap shared.Capability) error {
	n, err := r.queries.SetScheduleCompletedByUserID(ctx, sqlc.SetScheduleCompletedByUserIDParams{Completed: completed, ID: string(id), ActorID: string(actor)})
	if err != nil {
		return err
	}
	if n == 0 {
		return domain.ErrScheduleNotFound
	}
	return nil
}
func (r *ScheduleRepository) DeleteByUserID(ctx context.Context, actor domain.UserID, id domain.ScheduleID, cap shared.Capability) error {
	_, err := r.queries.DeleteScheduleByUserID(ctx, sqlc.DeleteScheduleByUserIDParams{ID: string(id), ActorID: string(actor)})
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrScheduleNotFound
	}
	return err
}
func (r *ScheduleRepository) TombstoneByUserID(ctx context.Context, actor domain.UserID, id domain.ScheduleID, cap shared.Capability) error {
	_, err := r.queries.TombstoneScheduleByUserID(ctx, sqlc.TombstoneScheduleByUserIDParams{ID: string(id), ActorID: string(actor)})
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrScheduleNotFound
	}
	return err
}
func (r *ScheduleRepository) DeleteUneditedFutureBySeries(ctx context.Context, actor domain.UserID, series domain.ScheduleID, from time.Time) error {
	_, err := r.queries.DeleteUneditedFutureSchedulesBySeries(ctx, sqlc.DeleteUneditedFutureSchedulesBySeriesParams{SeriesID: string(series), FromAt: pgTime(from), UserID: string(actor)})
	return err
}
func (r *ScheduleRepository) SetRecurrenceByUserID(ctx context.Context, actor domain.UserID, series domain.ScheduleID, anchor time.Time, weeks int, frequencies []dao.Frequency) error {
	if err := r.queries.ReplaceScheduleFrequenciesByUserID(ctx, sqlc.ReplaceScheduleFrequenciesByUserIDParams{SeriesID: string(series), UserID: string(actor)}); err != nil {
		return err
	}
	if err := r.queries.CreateScheduleFrequenciesByUserID(ctx, sqlc.CreateScheduleFrequenciesByUserIDParams{Frequencies: daoFrequencyValues(frequencies), SeriesID: string(series), UserID: string(actor)}); err != nil {
		return err
	}
	n, err := r.queries.SetScheduleRecurrenceByUserID(ctx, sqlc.SetScheduleRecurrenceByUserIDParams{FrequencyAnchorDate: pgDate(anchor), IntervalWeeks: int32(weeks), SeriesID: string(series), UserID: string(actor)})
	if err != nil {
		return err
	}
	if n == 0 {
		return domain.ErrScheduleNotFound
	}
	return nil
}
func (r *ScheduleRepository) StopRecurrenceByUserID(ctx context.Context, actor domain.UserID, series domain.ScheduleID) error {
	if err := r.queries.ClearScheduleFrequenciesByUserID(ctx, sqlc.ClearScheduleFrequenciesByUserIDParams{SeriesID: string(series), UserID: string(actor)}); err != nil {
		return err
	}
	n, err := r.queries.StopScheduleRecurrenceByUserID(ctx, sqlc.StopScheduleRecurrenceByUserIDParams{SeriesID: string(series), UserID: string(actor)})
	if err != nil {
		return err
	}
	if n == 0 {
		return domain.ErrScheduleNotFound
	}
	return nil
}
func (r *ScheduleRepository) UpdateSeriesTemplateByUserID(ctx context.Context, actor domain.UserID, series, snapshot domain.ScheduleID, s domain.Schedule) (dao.Schedule, error) {
	err := r.queries.SnapshotScheduleRootOccurrenceByUserID(ctx, sqlc.SnapshotScheduleRootOccurrenceByUserIDParams{ID: string(snapshot), SeriesID: string(series), UserID: string(actor)})
	if err != nil {
		return dao.Schedule{}, err
	}
	n, err := r.queries.UpdateScheduleSeriesTemplateByUserID(ctx, sqlc.UpdateScheduleSeriesTemplateByUserIDParams{Title: s.Title, Description: pgText(s.Description), Location: pgText(s.Location), StartAt: pgTime(s.StartAt), EndAt: pgTime(s.EndAt), Timezone: s.Timezone, SeriesID: string(series), UserID: string(actor)})
	if err != nil {
		return dao.Schedule{}, err
	}
	if n == 0 {
		return dao.Schedule{}, domain.ErrScheduleNotFound
	}
	row, err := r.queries.GetScheduleByUserIDForPermission(ctx, sqlc.GetScheduleByUserIDForPermissionParams{ID: string(series), ActorID: string(actor), ResourceID: string(shared.ResourceSchedule), Action: sqlc.Action(shared.ActionUpdate)})
	if err != nil {
		return dao.Schedule{}, err
	}
	return scheduleRow(row), nil
}
func (r *ScheduleRepository) UpsertOverrideByUserID(ctx context.Context, actor domain.UserID, s domain.Schedule) (string, error) {
	return r.queries.UpsertScheduleOverrideByUserID(ctx, sqlc.UpsertScheduleOverrideByUserIDParams{ID: string(s.ID), Title: s.Title, Description: pgText(s.Description), Location: pgText(s.Location), StartAt: pgTime(s.StartAt), EndAt: pgTime(s.EndAt), OccurrenceDate: pgDate(s.OccurrenceDate), Timezone: s.Timezone, Completed: s.Completed, Deleted: s.Deleted, SeriesID: string(s.SeriesID), UserID: string(actor)})
}
func (r *ScheduleRepository) ListSkippedOccurrences(ctx context.Context, actor domain.UserID, series domain.ScheduleID) ([]int64, error) {
	rows, err := r.queries.ListScheduleSkippedOccurrencesByUserID(ctx, sqlc.ListScheduleSkippedOccurrencesByUserIDParams{SeriesID: string(series), UserID: string(actor)})
	return dateRows(rows), err
}
func (r *ScheduleRepository) ListSkippedOccurrencesForPermission(ctx context.Context, actor domain.UserID, series domain.ScheduleID, cap shared.Capability) ([]int64, error) {
	rows, err := r.queries.ListScheduleSkippedOccurrencesForPermission(ctx, sqlc.ListScheduleSkippedOccurrencesForPermissionParams{SeriesID: string(series), ActorID: string(actor), ResourceID: string(cap.Resource), Action: sqlc.Action(cap.Action)})
	return dateRows(rows), err
}
func (r *ScheduleRepository) SetSkippedOccurrence(ctx context.Context, actor domain.UserID, series domain.ScheduleID, date time.Time, skipped bool) error {
	d := pgDate(date)
	if skipped {
		_, err := r.queries.SkipScheduleOccurrenceByUserID(ctx, sqlc.SkipScheduleOccurrenceByUserIDParams{ID: ulid.Make().String(), OccurrenceDate: d, SeriesID: string(series), UserID: string(actor)})
		return err
	}
	if err := r.queries.RestoreScheduleOccurrenceByUserID(ctx, sqlc.RestoreScheduleOccurrenceByUserIDParams{SeriesID: string(series), OccurrenceDate: d, UserID: string(actor)}); err != nil {
		return err
	}
	return r.queries.DeleteSkippedScheduleOccurrenceByUserID(ctx, sqlc.DeleteSkippedScheduleOccurrenceByUserIDParams{SeriesID: string(series), OccurrenceDate: d, UserID: string(actor)})
}
func (r *ScheduleRepository) SetAssigneeByUserID(ctx context.Context, actor domain.UserID, id domain.ScheduleID, assignee domain.UserID) error {
	n, err := r.queries.UpdateScheduleAssigneeByUserID(ctx, sqlc.UpdateScheduleAssigneeByUserIDParams{AssigneeID: string(assignee), SeriesID: string(id), ActorID: string(actor)})
	if err != nil {
		return err
	}
	if n == 0 {
		return domain.ErrScheduleNotFound
	}
	return nil
}
func (r *ScheduleRepository) SetProjectByUserID(ctx context.Context, actor domain.UserID, id domain.ScheduleID, project *domain.ProjectID) error {
	n, err := r.queries.UpdateScheduleProjectByUserID(ctx, sqlc.UpdateScheduleProjectByUserIDParams{SeriesID: string(id), ActorID: string(actor), ProjectID: pgTextPointer(project)})
	if err != nil {
		return err
	}
	if n == 0 {
		return domain.ErrScheduleNotFound
	}
	return nil
}
func (r *ScheduleRepository) ListEligibleAssignees(ctx context.Context, actor domain.UserID, id domain.ScheduleID) ([]dao.Assignee, error) {
	rows, err := r.queries.ListEligibleScheduleAssignees(ctx, sqlc.ListEligibleScheduleAssigneesParams{ID: string(id), ActorID: string(actor)})
	if err != nil {
		return nil, err
	}
	out := make([]dao.Assignee, 0, len(rows))
	for _, v := range rows {
		out = append(out, dao.Assignee{ID: v.ID, FirstName: v.FirstName, LastName: v.LastName, Email: v.Email, IsOwner: v.IsOwner})
	}
	return out, nil
}

func (r *ScheduleRepository) ListTags(ctx context.Context, actor domain.UserID, id domain.ScheduleID) ([]dao.ScheduleTag, error) {
	rows, err := r.queries.ListScheduleTagsByScheduleAndUserID(ctx, sqlc.ListScheduleTagsByScheduleAndUserIDParams{ScheduleID: string(id), UserID: string(actor)})
	if err != nil {
		return nil, err
	}
	out := make([]dao.ScheduleTag, 0, len(rows))
	for _, row := range rows {
		out = append(out, dao.ScheduleTag{ID: row.ID, UserID: row.UserID, Name: row.Name, CreatedAt: unix(row.CreatedAt), UpdatedAt: unix(row.UpdatedAt)})
	}
	return out, nil
}

func (r *ScheduleRepository) ListTagsForSchedules(ctx context.Context, actor domain.UserID, ids []domain.ScheduleID) (map[string][]dao.ScheduleTag, error) {
	result := make(map[string][]dao.ScheduleTag, len(ids))
	if len(ids) == 0 {
		return result, nil
	}
	requested := make([]string, 0, len(ids))
	for _, id := range ids {
		requested = append(requested, string(id))
		result[string(id)] = []dao.ScheduleTag{}
	}
	rows, err := r.queries.ListScheduleTagsByScheduleIDsAndUserID(ctx, sqlc.ListScheduleTagsByScheduleIDsAndUserIDParams{ScheduleIds: requested, UserID: string(actor)})
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		result[row.ScheduleID] = append(result[row.ScheduleID], dao.ScheduleTag{
			ID: row.ID, UserID: row.UserID, Name: row.Name, CreatedAt: unix(row.CreatedAt), UpdatedAt: unix(row.UpdatedAt),
		})
	}
	return result, nil
}

func (r *ScheduleRepository) AddTag(ctx context.Context, actor domain.UserID, id domain.ScheduleID, tagID string) error {
	result, err := r.queries.AddScheduleTagToScheduleByUserID(ctx, sqlc.AddScheduleTagToScheduleByUserIDParams{ScheduleID: string(id), TagID: tagID, UserID: string(actor)})
	if err != nil {
		return err
	}
	if !result.Owned {
		return usecase.ErrScheduleTagNotFound
	}
	return nil
}

func (r *ScheduleRepository) RemoveTag(ctx context.Context, actor domain.UserID, id domain.ScheduleID, tagID string) error {
	result, err := r.queries.RemoveScheduleTagFromScheduleByUserID(ctx, sqlc.RemoveScheduleTagFromScheduleByUserIDParams{ScheduleID: string(id), TagID: tagID, UserID: string(actor)})
	if err != nil {
		return err
	}
	if !result.Owned {
		return usecase.ErrScheduleTagNotFound
	}
	return nil
}

func (r *ScheduleRepository) DeleteProjectSchedulesByActor(ctx context.Context, actor domain.UserID, project domain.ProjectID) error {
	_, err := r.queries.DeleteProjectSchedulesByActor(ctx, sqlc.DeleteProjectSchedulesByActorParams{ActorID: string(actor), ProjectID: string(project)})
	return err
}

func (r *ScheduleRepository) ReassignProjectMemberSchedules(ctx context.Context, actor domain.UserID, project domain.ProjectID, member domain.UserID) error {
	_, err := r.queries.ReassignProjectMemberSchedulesByActor(ctx, sqlc.ReassignProjectMemberSchedulesByActorParams{ActorID: string(actor), ProjectID: string(project), MemberID: string(member)})
	return err
}

func scheduleRow(v interface{}) dao.Schedule {
	switch x := v.(type) {
	case sqlc.GetScheduleByUserIDForPermissionRow:
		return buildSchedule(x.ID, x.UserID, x.ProjectID, x.AssigneeID, x.Title, x.Description, x.Location, x.IntervalWeeks, x.RepeatState, x.FrequencyAnchorDate, x.SeriesID, x.OccurrenceDate, x.Timezone, x.IsException, x.Completed, x.Deleted, x.Frequencies, x.StartAt, x.EndAt, x.CreatedAt, x.UpdatedAt, x.Revision, x.ChangedBy)
	case sqlc.CreateScheduleByUserIDRow:
		return buildSchedule(x.ID, x.UserID, x.ProjectID, x.AssigneeID, x.Title, x.Description, x.Location, x.IntervalWeeks, x.RepeatState, x.FrequencyAnchorDate, x.SeriesID, x.OccurrenceDate, x.Timezone, x.IsException, x.Completed, x.Deleted, x.Frequencies, x.StartAt, x.EndAt, x.CreatedAt, x.UpdatedAt, x.Revision, x.ChangedBy)
	case sqlc.UpdateScheduleByUserIDRow:
		return buildSchedule(x.ID, x.UserID, x.ProjectID, x.AssigneeID, x.Title, x.Description, x.Location, x.IntervalWeeks, x.RepeatState, x.FrequencyAnchorDate, x.SeriesID, x.OccurrenceDate, x.Timezone, x.IsException, x.Completed, x.Deleted, x.Frequencies, x.StartAt, x.EndAt, x.CreatedAt, x.UpdatedAt, x.Revision, x.ChangedBy)
	case sqlc.ListSchedulesForOccurrenceCommandByUserIDRow:
		return buildSchedule(x.ID, x.UserID, x.ProjectID, x.AssigneeID, x.Title, x.Description, x.Location, x.IntervalWeeks, x.RepeatState, x.FrequencyAnchorDate, x.SeriesID, x.OccurrenceDate, x.Timezone, x.IsException, x.Completed, x.Deleted, x.Frequencies, x.StartAt, x.EndAt, x.CreatedAt, x.UpdatedAt, x.Revision, x.ChangedBy)
	case sqlc.ListSchedulesForOccurrenceProjectionByAssigneeUserIDRow:
		row := buildSchedule(x.ID, x.UserID, x.ProjectID, x.AssigneeID, x.Title, x.Description, x.Location, x.IntervalWeeks, x.RepeatState, x.FrequencyAnchorDate, x.SeriesID, x.OccurrenceDate, x.Timezone, x.IsException, x.Completed, x.Deleted, x.Frequencies, x.StartAt, x.EndAt, x.CreatedAt, x.UpdatedAt, x.Revision, x.ChangedBy)
		row.ProjectDone = boolValue(x.ProjectDone)
		return row
	case sqlc.ListSchedulesForOccurrenceProjectionByProjectAndUserIDRow:
		row := buildSchedule(x.ID, x.UserID, x.ProjectID, x.AssigneeID, x.Title, x.Description, x.Location, x.IntervalWeeks, x.RepeatState, x.FrequencyAnchorDate, x.SeriesID, x.OccurrenceDate, x.Timezone, x.IsException, x.Completed, x.Deleted, x.Frequencies, x.StartAt, x.EndAt, x.CreatedAt, x.UpdatedAt, x.Revision, x.ChangedBy)
		row.ProjectDone = boolValue(x.ProjectDone)
		return row
	case sqlc.ListActiveScheduleSeriesByAssigneeUserIDRow:
		return buildSchedule(x.ID, x.UserID, x.ProjectID, x.AssigneeID, x.Title, x.Description, x.Location, x.IntervalWeeks, x.RepeatState, x.FrequencyAnchorDate, x.SeriesID, x.OccurrenceDate, x.Timezone, x.IsException, x.Completed, x.Deleted, x.Frequencies, x.StartAt, x.EndAt, x.CreatedAt, x.UpdatedAt, x.Revision, x.ChangedBy)
	default:
		panic(fmt.Sprintf("unsupported schedule sql row %T", v))
	}
}
func buildSchedule(id, user string, project pgtype.Text, assignee, title string, description, location pgtype.Text, weeks int32, state pgtype.Text, anchor pgtype.Date, series string, occurrence pgtype.Date, tz string, exception, completed bool, deleted any, freq []string, start, end, created, updated pgtype.Timestamptz, revision int32, changedBy string) dao.Schedule {
	return dao.Schedule{ID: id, UserID: user, ProjectID: nullableText(project), AssigneeID: assignee, Title: title, Description: nullableText(description), Location: nullableText(location), IntervalWeeks: int(weeks), Frequencies: frequencyDAO(freq), RepeatState: nullableText(state), FrequencyAnchorDate: dateUnix(anchor), SeriesID: series, OccurrenceDate: dateString(occurrence), Timezone: tz, IsException: exception, Completed: completed, Deleted: boolValue(deleted), StartAt: unix(start), EndAt: unix(end), CreatedAt: unix(created), UpdatedAt: unix(updated), Revision: revision, ChangedBy: changedBy, CursorStartAt: formatTime(start)}
}
func frequencyDAO(values []string) []dao.Frequency {
	out := make([]dao.Frequency, 0, len(values))
	for _, v := range values {
		out = append(out, dao.Frequency{Value: v})
	}
	return out
}
func frequencyValues(values []domain.Frequency) []string {
	out := make([]string, 0, len(values))
	for _, v := range values {
		out = append(out, v.Value)
	}
	return out
}
func daoFrequencyValues(values []dao.Frequency) []string {
	out := make([]string, 0, len(values))
	for _, v := range values {
		out = append(out, v.Value)
	}
	return out
}
func pgText(s string) pgtype.Text {
	if s == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{String: s, Valid: true}
}
func pgTextPointer(p *domain.ProjectID) pgtype.Text {
	if p == nil {
		return pgtype.Text{}
	}
	return pgtype.Text{String: string(*p), Valid: true}
}
func pgDate(t time.Time) pgtype.Date {
	if t.IsZero() {
		return pgtype.Date{}
	}
	return pgtype.Date{Time: t, Valid: true}
}
func pgTime(t time.Time) pgtype.Timestamptz {
	if t.IsZero() {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: t, Valid: true}
}
func nullableText(v pgtype.Text) string {
	if v.Valid {
		return v.String
	}
	return ""
}
func unix(v pgtype.Timestamptz) int64 {
	if v.Valid {
		return v.Time.Unix()
	}
	return 0
}
func formatTime(v pgtype.Timestamptz) string {
	if v.Valid {
		return v.Time.UTC().Format(time.RFC3339Nano)
	}
	return ""
}
func dateUnix(v pgtype.Date) int64 {
	if v.Valid {
		return v.Time.UTC().Unix()
	}
	return 0
}
func dateString(v pgtype.Date) string {
	if v.Valid {
		return v.Time.Format("2006-01-02")
	}
	return ""
}
func boolValue(v any) bool {
	switch x := v.(type) {
	case bool:
		return x
	case *bool:
		return x != nil && *x
	case int64:
		return x != 0
	case int32:
		return x != 0
	case []byte:
		return string(x) == "t" || string(x) == "true"
	}
	return false
}
func dateRows(rows []pgtype.Date) []int64 {
	out := make([]int64, 0, len(rows))
	for _, v := range rows {
		out = append(out, dateUnix(v))
	}
	return out
}
