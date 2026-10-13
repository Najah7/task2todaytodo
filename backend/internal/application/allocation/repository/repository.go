package repository

import (
	"context"
	"sort"
	"time"

	"github.com/Najah7/task2todaytodo/db/sqlc"
	"github.com/Najah7/task2todaytodo/internal/application/allocation/dao"
	"github.com/Najah7/task2todaytodo/internal/application/allocation/domain"
	allocationusecase "github.com/Najah7/task2todaytodo/internal/application/allocation/usecase"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

var _ allocationusecase.Repository = (*AllocationRepository)(nil)

type AllocationRepository struct {
	queries *sqlc.Queries
	pool    *pgxpool.Pool
	tx      pgx.Tx
}

func NewAllocationRepository(pool *pgxpool.Pool) *AllocationRepository {
	return &AllocationRepository{queries: sqlc.New(pool), pool: pool}
}

// NewAllocationRepositoryFromTx binds allocation projections to an existing
// transaction without requiring a pool-backed repository instance.
func NewAllocationRepositoryFromTx(tx pgx.Tx) *AllocationRepository {
	return &AllocationRepository{queries: sqlc.New(tx), tx: tx}
}

func (r *AllocationRepository) WithTx(tx pgx.Tx) *AllocationRepository {
	return &AllocationRepository{queries: r.queries.WithTx(tx), pool: r.pool, tx: tx}
}

func (r *AllocationRepository) LockProjectsForMutation(ctx context.Context, userID string, projectIDs []string) error {
	unique := make(map[string]struct{}, len(projectIDs))
	ids := make([]string, 0, len(projectIDs))
	for _, id := range projectIDs {
		if id == "" {
			return allocationusecase.ErrInvalidRequest
		}
		if _, exists := unique[id]; exists {
			continue
		}
		unique[id] = struct{}{}
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		if _, err := r.queries.LockProjectForLifecycle(ctx, id); err != nil {
			return allocationusecase.ErrProjectUnavailable
		}
		if _, err := r.queries.GetProjectAllocationProject(ctx, sqlc.GetProjectAllocationProjectParams{ProjectID: id, UserID: userID}); err != nil {
			return allocationusecase.ErrProjectUnavailable
		}
	}
	return nil
}

func (r *AllocationRepository) LockUser(ctx context.Context, userID string) error {
	if userID == "" {
		return allocationusecase.ErrInvalidRequest
	}
	return r.queries.LockAllocationUser(ctx, userID)
}

func pgDate(value time.Time) pgtype.Date {
	return pgtype.Date{Time: time.Date(value.Year(), value.Month(), value.Day(), 0, 0, 0, 0, time.UTC), Valid: true}
}

func pgInstant(value time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: value.UTC(), Valid: true}
}

func pgWallTime(value time.Time) pgtype.Time {
	return pgtype.Time{Microseconds: int64(value.Hour())*3_600_000_000 + int64(value.Minute())*60_000_000 + int64(value.Second())*1_000_000 + int64(value.Nanosecond()/1_000), Valid: true}
}

func wallString(value pgtype.Time) string {
	if !value.Valid {
		return ""
	}
	return time.Date(0, 1, 1, int(value.Microseconds/3_600_000_000), int(value.Microseconds/60_000_000%60), 0, 0, time.UTC).Format("15:04")
}

func dateValue(value pgtype.Date) time.Time {
	if !value.Valid {
		return time.Time{}
	}
	return time.Date(value.Time.Year(), value.Time.Month(), value.Time.Day(), 0, 0, 0, 0, time.UTC)
}

func (r *AllocationRepository) ListPlans(ctx context.Context, userID string) ([]dao.PlanItem, error) {
	rows, err := r.queries.ListProjectAllocationPlans(ctx, userID)
	if err != nil {
		return nil, err
	}
	items := make([]dao.PlanItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, dao.PlanItem{ProjectID: row.ProjectID, Title: row.Title, TargetMinutes: int(row.TargetMinutes)})
	}
	return items, nil
}

func (r *AllocationRepository) GetProfile(ctx context.Context, userID string) (dao.Profile, error) {
	row, err := r.queries.GetAllocationActiveTimeProfile(ctx, userID)
	if err != nil {
		return dao.Profile{}, err
	}
	return dao.Profile{Timezone: row.Timezone, ActiveStartTime: wallString(row.ActiveStartTime), ActiveEndTime: wallString(row.ActiveEndTime)}, nil
}

func (r *AllocationRepository) GetProjectStatus(ctx context.Context, userID, projectID string) (string, error) {
	row, err := r.queries.GetProjectAllocationProject(ctx, sqlc.GetProjectAllocationProjectParams{ProjectID: projectID, UserID: userID})
	if err == pgx.ErrNoRows {
		return "", allocationusecase.ErrProjectUnavailable
	}
	if err != nil {
		return "", err
	}
	return row.Status, nil
}

func (r *AllocationRepository) ListProjects(ctx context.Context, userID string) ([]dao.Project, error) {
	rows, err := r.queries.ListAllocationProjects(ctx, userID)
	if err != nil {
		return nil, err
	}
	projects := make([]dao.Project, 0, len(rows))
	for _, row := range rows {
		project := dao.Project{ID: row.ID, Title: row.Title}
		if row.TargetMinutes.Valid {
			project.TargetMinutes = int(row.TargetMinutes.Int32)
		}
		projects = append(projects, project)
	}
	return projects, nil
}

func (r *AllocationRepository) ListRules(ctx context.Context, userID string) ([]dao.Rule, error) {
	rows, err := r.queries.ListProjectAllocationRules(ctx, userID)
	if err != nil {
		return nil, err
	}
	rules := make([]dao.Rule, 0, len(rows))
	for _, row := range rows {
		paused := ""
		if row.PauseReason.Valid {
			paused = row.PauseReason.String
		}
		rule := dao.Rule{ID: row.RuleID, ProjectID: row.ProjectID, ProjectTitle: row.ProjectTitle, Revision: row.Revision, SourceRevisionID: row.SourceRevisionID, Weekday: int(row.Weekday), StartTime: wallString(row.LocalStart), EndTime: wallString(row.LocalEnd), EndDayOffset: int(row.EndDayOffset), Timezone: row.Timezone, Frequency: row.Frequency, EffectiveFrom: dateValue(row.EffectiveFrom), EffectiveAt: row.EffectiveAt.Time, PausedReason: paused}
		if row.PausedAt.Valid {
			pausedAt := row.PausedAt.Time
			rule.PausedAt = &pausedAt
		}
		if row.StoppedAfter.Valid {
			stopped := dateValue(row.StoppedAfter)
			rule.StoppedAfter = &stopped
		}
		rules = append(rules, rule)
	}
	return rules, nil
}

func (r *AllocationRepository) ListPauseIntervals(ctx context.Context, ruleID string) ([]dao.PauseInterval, error) {
	rows, err := r.queries.ListAllocationPauseIntervals(ctx, ruleID)
	if err != nil {
		return nil, err
	}
	result := make([]dao.PauseInterval, 0, len(rows))
	for _, row := range rows {
		item := dao.PauseInterval{PausedAt: row.PausedAt.Time}
		if row.ResumedAt.Valid {
			resumed := row.ResumedAt.Time
			item.ResumedAt = &resumed
		}
		result = append(result, item)
	}
	return result, nil
}

func (r *AllocationRepository) ListAllocations(ctx context.Context, userID string, from, through time.Time) ([]dao.Allocation, error) {
	rows, err := r.queries.ListAllocationProjectFrames(ctx, sqlc.ListAllocationProjectFramesParams{UserID: userID, ThroughAt: pgInstant(through), FromAt: pgInstant(from)})
	if err != nil {
		return nil, err
	}
	projects, err := r.ListProjects(ctx, userID)
	if err != nil {
		return nil, err
	}
	titles := make(map[string]string, len(projects))
	for _, project := range projects {
		titles[project.ID] = project.Title
	}
	allocations := make([]dao.Allocation, 0, len(rows))
	currentRules, err := r.ListRuleRevisionsForProjection(ctx, userID)
	if err != nil {
		return nil, err
	}
	type ruleState struct {
		revision int32
		stopped  bool
	}
	ruleRevisions := make(map[string]ruleState, len(currentRules))
	for _, rule := range currentRules {
		state, exists := ruleRevisions[rule.ID]
		if !exists || rule.CurrentRevision > state.revision {
			ruleRevisions[rule.ID] = ruleState{revision: rule.CurrentRevision, stopped: rule.DeletedAt != nil}
		}
	}
	for _, row := range rows {
		projectID, startAt, endAt, timezone, sourceID := row.ProjectID, row.StartAt.Time, row.EndAt.Time, row.Timezone, row.SourceRevisionID
		isException := row.ExceptionSourceRevisionID.Valid
		if row.ExceptionProjectID.Valid {
			projectID = row.ExceptionProjectID.String
		}
		if row.ExceptionStartAt.Valid {
			startAt = row.ExceptionStartAt.Time
		}
		if row.ExceptionEndAt.Valid {
			endAt = row.ExceptionEndAt.Time
		}
		if row.ExceptionTimezone.Valid {
			timezone = row.ExceptionTimezone.String
		}
		if row.ExceptionSourceRevisionID.Valid {
			sourceID = row.ExceptionSourceRevisionID.String
		}
		state := ruleRevisions[row.RuleID]
		allocations = append(allocations, dao.Allocation{ID: row.ID, RuleID: row.RuleID, OccurrenceDate: dateValue(row.OccurrenceDate), SourceRevisionID: sourceID, ProjectID: projectID, ProjectTitle: titles[projectID], Timezone: timezone, StartAt: startAt.UTC(), EndAt: endAt.UTC(), Revision: row.Revision, RuleRevision: state.revision, IsException: isException, CanUpdate: true, CanDelete: true, CanUpdateFollowing: !state.stopped, CanDeleteFollowing: !state.stopped})
	}
	return allocations, nil
}

func (r *AllocationRepository) ListActiveTimeExceptions(ctx context.Context, userID string, fromDate, throughDate time.Time) ([]dao.ActiveTimeException, error) {
	rows, err := r.queries.ListDailyActiveTimeExceptions(ctx, sqlc.ListDailyActiveTimeExceptionsParams{UserID: userID, FromDate: pgDate(fromDate), ThroughDate: pgDate(throughDate)})
	if err != nil {
		return nil, err
	}
	exceptions := make([]dao.ActiveTimeException, 0, len(rows))
	for _, row := range rows {
		exceptions = append(exceptions, dao.ActiveTimeException{LocalDate: dateValue(row.LocalDate), ActiveStartLocal: row.ActiveStartLocal.Time, ActiveEndLocal: row.ActiveEndLocal.Time, Timezone: row.Timezone, Revision: row.Revision, HasException: true})
	}
	return exceptions, nil
}

func (r *AllocationRepository) SetPlan(ctx context.Context, userID, projectID string, minutes int) error {
	return r.queries.SetProjectAllocationPlan(ctx, sqlc.SetProjectAllocationPlanParams{UserID: userID, ProjectID: projectID, TargetMinutes: int32(minutes)})
}

func (r *AllocationRepository) DeletePlan(ctx context.Context, userID, projectID string) error {
	return r.queries.DeleteProjectAllocationPlan(ctx, sqlc.DeleteProjectAllocationPlanParams{UserID: userID, ProjectID: projectID})
}

func (r *AllocationRepository) CreateRule(ctx context.Context, userID, ruleID, projectID string) error {
	_, err := r.queries.CreateProjectTimeAllocationRule(ctx, sqlc.CreateProjectTimeAllocationRuleParams{ID: ruleID, UserID: userID, ProjectID: projectID})
	return err
}

func (r *AllocationRepository) CreateRuleRevision(ctx context.Context, userID, ruleID, projectID, sourceID string, revision int32, effectiveFrom, recurrenceStartDate, effectiveAt time.Time, slot domain.Slot) error {
	_, err := r.queries.CreateProjectTimeAllocationRuleRevision(ctx, sqlc.CreateProjectTimeAllocationRuleRevisionParams{ID: sourceID, RuleID: ruleID, UserID: userID, ProjectID: projectID, Revision: revision, EffectiveFrom: pgDate(effectiveFrom), RecurrenceStartDate: pgDate(recurrenceStartDate), EffectiveAt: pgInstant(effectiveAt), Weekday: int16(slot.Weekday), LocalStart: pgWallTime(slot.Start), LocalEnd: pgWallTime(slot.End), EndDayOffset: int16(slot.EndDayOffset), Timezone: slot.Timezone, Frequency: slot.Frequency, ChangedBy: userID})
	return err
}

func (r *AllocationRepository) SupersedeFutureRevisions(ctx context.Context, ruleID string, cutoff time.Time) error {
	return r.queries.SupersedeFutureAllocationRevisions(ctx, sqlc.SupersedeFutureAllocationRevisionsParams{RuleID: ruleID, EffectiveAt: pgInstant(cutoff)})
}

func (r *AllocationRepository) StopRule(ctx context.Context, userID, ruleID string, cutoff time.Time) error {
	return r.queries.StopProjectAllocationRule(ctx, sqlc.StopProjectAllocationRuleParams{RuleID: ruleID, UserID: userID, Cutoff: pgInstant(cutoff)})
}

func (r *AllocationRepository) StopFollowingRule(ctx context.Context, userID, ruleID string, cutoff, lastOrigin time.Time) error {
	return r.queries.StopFollowingProjectAllocationRule(ctx, sqlc.StopFollowingProjectAllocationRuleParams{
		RuleID: ruleID, UserID: userID, Cutoff: pgInstant(cutoff), StoppedAfter: pgDate(lastOrigin),
	})
}

func (r *AllocationRepository) SetRulePause(ctx context.Context, userID, ruleID, reason string, at time.Time) error {
	var pause pgtype.Text
	if reason != "" {
		pause = pgtype.Text{String: reason, Valid: true}
	}
	return r.queries.SetAllocationPause(ctx, sqlc.SetAllocationPauseParams{PauseReason: pause, PausedAt: pgInstant(at), RuleID: ruleID, UserID: userID})
}

func (r *AllocationRepository) StartPauseInterval(ctx context.Context, projectID, ruleID string, at time.Time) error {
	return r.queries.StartAllocationPauseInterval(ctx, sqlc.StartAllocationPauseIntervalParams{ProjectID: projectID, RuleID: ruleID, PausedAt: pgInstant(at)})
}

func (r *AllocationRepository) ClosePauseInterval(ctx context.Context, projectID, ruleID string, at time.Time) error {
	return r.queries.CloseProjectAllocationPauseInterval(ctx, sqlc.CloseProjectAllocationPauseIntervalParams{ProjectID: projectID, RuleID: ruleID, ResumedAt: pgInstant(at)})
}

func (r *AllocationRepository) DeleteFutureRows(ctx context.Context, ruleID string, cutoff time.Time, userID string, includeExceptions bool) error {
	if includeExceptions {
		if err := r.queries.DeleteFutureProjectAllocationRuleRows(ctx, sqlc.DeleteFutureProjectAllocationRuleRowsParams{ChangedBy: userID, RuleID: ruleID, Cutoff: pgInstant(cutoff)}); err != nil {
			return err
		}
		return r.queries.DeleteFutureProjectAllocationExceptions(ctx, sqlc.DeleteFutureProjectAllocationExceptionsParams{ChangedBy: userID, RuleID: ruleID, Cutoff: pgInstant(cutoff)})
	}
	return r.queries.DeleteFutureProjectAllocations(ctx, sqlc.DeleteFutureProjectAllocationsParams{ChangedBy: userID, RuleID: ruleID, Cutoff: pgInstant(cutoff)})
}

func (r *AllocationRepository) DeleteFollowingOrigins(ctx context.Context, userID, ruleID string, originDate, cutoff time.Time) error {
	if err := r.queries.DeleteFollowingProjectAllocationRowsByOrigin(ctx, sqlc.DeleteFollowingProjectAllocationRowsByOriginParams{
		ChangedBy: userID, RuleID: ruleID, OriginDate: pgDate(originDate), Cutoff: pgInstant(cutoff),
	}); err != nil {
		return err
	}
	return r.queries.DeleteFollowingProjectAllocationExceptionsByOrigin(ctx, sqlc.DeleteFollowingProjectAllocationExceptionsByOriginParams{
		ChangedBy: userID, RuleID: ruleID, OriginDate: pgDate(originDate), Cutoff: pgInstant(cutoff),
	})
}

func (r *AllocationRepository) DeleteFutureOrdinaryOriginsAfter(ctx context.Context, userID, ruleID string, originDate, cutoff time.Time) error {
	return r.queries.DeleteFutureOrdinaryAllocationOriginsAfter(ctx, sqlc.DeleteFutureOrdinaryAllocationOriginsAfterParams{
		ChangedBy: userID, RuleID: ruleID, OriginDate: pgDate(originDate), Cutoff: pgInstant(cutoff),
	})
}

func (r *AllocationRepository) InsertOccurrence(ctx context.Context, userID string, value domain.Occurrence) (dao.Allocation, error) {
	row, err := r.queries.InsertProjectTimeAllocation(ctx, sqlc.InsertProjectTimeAllocationParams{ID: value.ID, RuleID: value.RuleID, SourceRevisionID: value.SourceRevisionID, UserID: userID, ProjectID: value.ProjectID, OccurrenceDate: pgDate(value.OccurrenceDate), StartAt: pgInstant(value.StartAt), EndAt: pgInstant(value.EndAt), Timezone: value.Timezone, ChangedBy: userID})
	if err != nil {
		return dao.Allocation{}, err
	}
	return r.allocationFromRow(ctx, userID, row.ID, row.RuleID, row.SourceRevisionID, row.ProjectID, row.OccurrenceDate, row.StartAt, row.EndAt, row.Timezone, row.Revision, false)
}

func (r *AllocationRepository) RefreshOccurrence(ctx context.Context, userID string, value domain.Occurrence, cutoff time.Time) error {
	_, err := r.queries.UpsertProjectTimeAllocationProjection(ctx, sqlc.UpsertProjectTimeAllocationProjectionParams{
		ID: value.ID, RuleID: value.RuleID, SourceRevisionID: value.SourceRevisionID, UserID: userID,
		ProjectID: value.ProjectID, OccurrenceDate: pgDate(value.OccurrenceDate), StartAt: pgInstant(value.StartAt),
		EndAt: pgInstant(value.EndAt), Timezone: value.Timezone, ChangedBy: userID, Cutoff: pgInstant(cutoff),
	})
	if err == pgx.ErrNoRows {
		// A concrete exception or a row that has already started is authoritative.
		return nil
	}
	return err
}

func (r *AllocationRepository) GetOccurrence(ctx context.Context, ruleID, userID string, date time.Time) (dao.Allocation, error) {
	row, err := r.queries.GetAllocationOccurrenceForMutation(ctx, sqlc.GetAllocationOccurrenceForMutationParams{RuleID: ruleID, UserID: userID, OccurrenceDate: pgDate(date)})
	if err == pgx.ErrNoRows {
		return dao.Allocation{}, allocationusecase.ErrNotFound
	}
	if err != nil {
		return dao.Allocation{}, err
	}
	sourceID := row.SourceRevisionID
	if row.ExceptionSourceRevisionID.Valid {
		sourceID = row.ExceptionSourceRevisionID.String
	}
	return r.allocationFromRow(ctx, userID, row.ID, row.RuleID, sourceID, row.EffectiveProjectID, row.OccurrenceDate, row.StartAt, row.EndAt, row.Timezone, row.Revision, row.ExceptionSourceRevisionID.Valid)
}

func (r *AllocationRepository) ReadOccurrence(ctx context.Context, ruleID, userID string, date time.Time) (dao.Allocation, error) {
	row, err := r.queries.ReadAllocationOccurrence(ctx, sqlc.ReadAllocationOccurrenceParams{RuleID: ruleID, UserID: userID, OccurrenceDate: pgDate(date)})
	if err == pgx.ErrNoRows {
		return dao.Allocation{}, allocationusecase.ErrNotFound
	}
	if err != nil {
		return dao.Allocation{}, err
	}
	sourceID := row.SourceRevisionID
	if row.ExceptionSourceRevisionID.Valid {
		sourceID = row.ExceptionSourceRevisionID.String
	}
	return r.allocationFromRow(ctx, userID, row.ID, row.RuleID, sourceID, row.EffectiveProjectID, row.OccurrenceDate, row.StartAt, row.EndAt, row.Timezone, row.Revision, row.ExceptionSourceRevisionID.Valid)
}

func (r *AllocationRepository) MaterializeOccurrence(ctx context.Context, userID string, value domain.Occurrence, cutoff time.Time) (dao.Allocation, error) {
	if err := r.RefreshOccurrence(ctx, userID, value, cutoff); err != nil {
		return dao.Allocation{}, err
	}
	return r.GetOccurrence(ctx, value.RuleID, userID, value.OccurrenceDate)
}

func (r *AllocationRepository) SaveOccurrenceException(ctx context.Context, userID string, value domain.Occurrence, expected int32, deleted bool) error {
	var deletedAt pgtype.Timestamptz
	if deleted {
		deletedAt = pgInstant(time.Now().UTC())
	}
	if err := r.queries.UpsertProjectTimeAllocationException(ctx, sqlc.UpsertProjectTimeAllocationExceptionParams{RuleID: value.RuleID, OccurrenceDate: pgDate(value.OccurrenceDate), SourceRevisionID: value.SourceRevisionID, UserID: userID, ProjectID: value.ProjectID, StartAt: pgInstant(value.StartAt), EndAt: pgInstant(value.EndAt), Timezone: value.Timezone, DeletedAt: deletedAt, ChangedBy: userID}); err != nil {
		return err
	}
	rows, err := r.queries.UpdateProjectTimeAllocationProjectionFromException(ctx, sqlc.UpdateProjectTimeAllocationProjectionFromExceptionParams{SourceRevisionID: value.SourceRevisionID, ProjectID: value.ProjectID, StartAt: pgInstant(value.StartAt), EndAt: pgInstant(value.EndAt), Timezone: value.Timezone, DeletedAt: deletedAt, ChangedBy: userID, RuleID: value.RuleID, OccurrenceDate: pgDate(value.OccurrenceDate), ExpectedRevision: expected})
	if err != nil {
		return err
	}
	if rows == 0 {
		return allocationusecase.ErrPreconditionFailed
	}
	return nil
}

func (r *AllocationRepository) SaveActiveTimeException(ctx context.Context, userID string, expected int32, value domain.ActiveTimeException) (dao.ActiveTimeException, error) {
	current, err := r.queries.GetDailyActiveTimeException(ctx, sqlc.GetDailyActiveTimeExceptionParams{UserID: userID, LocalDate: pgDate(value.LocalDate)})
	if err == pgx.ErrNoRows {
		if expected != 0 {
			return dao.ActiveTimeException{}, allocationusecase.ErrPreconditionFailed
		}
	} else if err != nil {
		return dao.ActiveTimeException{}, err
	} else if current.Revision != expected {
		return dao.ActiveTimeException{}, allocationusecase.ErrPreconditionFailed
	}
	row, err := r.queries.CreateDailyActiveTimeException(ctx, sqlc.CreateDailyActiveTimeExceptionParams{UserID: userID, LocalDate: pgDate(value.LocalDate), ActiveStartLocal: pgtype.Timestamp{Time: value.ActiveStartLocal, Valid: true}, ActiveEndLocal: pgtype.Timestamp{Time: value.ActiveEndLocal, Valid: true}, Timezone: value.Timezone})
	if err != nil {
		return dao.ActiveTimeException{}, err
	}
	return dao.ActiveTimeException{LocalDate: dateValue(row.LocalDate), ActiveStartLocal: row.ActiveStartLocal.Time, ActiveEndLocal: row.ActiveEndLocal.Time, Timezone: row.Timezone, Revision: row.Revision, HasException: true}, nil
}

func (r *AllocationRepository) DeleteActiveTimeException(ctx context.Context, userID string, date time.Time, revision int32) error {
	rows, err := r.queries.DeleteDailyActiveTimeException(ctx, sqlc.DeleteDailyActiveTimeExceptionParams{UserID: userID, LocalDate: pgDate(date), Revision: revision})
	if err != nil {
		return err
	}
	if rows == 0 {
		return allocationusecase.ErrPreconditionFailed
	}
	return nil
}

func (r *AllocationRepository) FindAllocationConflict(ctx context.Context, userID string, start, end time.Time, excludeRule string, excludeDate time.Time) (dao.Allocation, error) {
	var ruleArg pgtype.Text
	var date pgtype.Date
	if excludeRule != "" {
		ruleArg = pgtype.Text{String: excludeRule, Valid: true}
		date = pgDate(excludeDate)
	}
	row, err := r.queries.FindAllocationOverlap(ctx, sqlc.FindAllocationOverlapParams{UserID: userID, ThroughAt: pgInstant(end), FromAt: pgInstant(start), ExcludeRuleID: ruleArg, ExcludeOccurrenceDate: date})
	if err == pgx.ErrNoRows {
		return dao.Allocation{}, allocationusecase.ErrNoConflict
	}
	if err != nil {
		return dao.Allocation{}, err
	}
	return dao.Allocation{RuleID: row.RuleID, OccurrenceDate: dateValue(row.OccurrenceDate), ProjectID: row.ProjectID, StartAt: row.StartAt.Time, EndAt: row.EndAt.Time}, nil
}

func (r *AllocationRepository) ListExceptionOrigins(ctx context.Context, userID string, from, through time.Time) (map[string]bool, error) {
	rows, err := r.queries.ListAllocationExceptionOrigins(ctx, sqlc.ListAllocationExceptionOriginsParams{UserID: userID, FromDate: pgDate(from), ThroughDate: pgDate(through)})
	if err != nil {
		return nil, err
	}
	result := make(map[string]bool, len(rows))
	for _, row := range rows {
		result[row.RuleID+"|"+dateValue(row.OccurrenceDate).Format("2006-01-02")] = true
	}
	return result, nil
}

func (r *AllocationRepository) ListUserIDs(ctx context.Context) ([]string, error) {
	return r.queries.ListAllocationUserIDs(ctx)
}

func (r *AllocationRepository) allocationFromRow(ctx context.Context, userID, id, ruleID, sourceID, projectID string, date pgtype.Date, start, end pgtype.Timestamptz, timezone string, revision int32, isException bool) (dao.Allocation, error) {
	value := dao.Allocation{ID: id, RuleID: ruleID, OccurrenceDate: dateValue(date), SourceRevisionID: sourceID, ProjectID: projectID, Timezone: timezone, StartAt: start.Time.UTC(), EndAt: end.Time.UTC(), Revision: revision, IsException: isException, CanUpdate: true, CanDelete: true, CanUpdateFollowing: true, CanDeleteFollowing: true}
	title, err := r.queries.GetProjectAllocationProject(ctx, sqlc.GetProjectAllocationProjectParams{ProjectID: projectID, UserID: userID})
	if err == pgx.ErrNoRows {
		return dao.Allocation{}, allocationusecase.ErrNotFound
	}
	if err != nil {
		return dao.Allocation{}, err
	}
	value.ProjectTitle = title.Title
	rules, err := r.ListRuleRevisionsForProjection(ctx, userID)
	if err != nil {
		return dao.Allocation{}, err
	}
	for _, rule := range rules {
		if rule.ID == ruleID && rule.CurrentRevision > value.RuleRevision {
			value.RuleRevision = rule.CurrentRevision
			value.CanUpdateFollowing = rule.DeletedAt == nil
			value.CanDeleteFollowing = rule.DeletedAt == nil
		}
	}
	return value, nil
}
