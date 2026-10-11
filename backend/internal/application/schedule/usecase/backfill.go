package usecase

import (
	"context"
	"errors"
	"sort"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/schedule/dao"
	"github.com/Najah7/task2todaytodo/internal/application/schedule/domain"
	"github.com/Najah7/task2todaytodo/internal/application/shared"
	"github.com/Najah7/task2todaytodo/internal/application/shared/recurrence"
)

var ErrBackfillProjectSetChanged = errors.New("schedule project associations changed during backfill")

type BackfillUseCase struct {
	uow UOW
	ids shared.ID
}

func NewBackfillUseCase(uow UOW, ids shared.ID) *BackfillUseCase {
	return &BackfillUseCase{uow: uow, ids: ids}
}

func (uc *BackfillUseCase) Execute(ctx context.Context, actorID string, cutoff time.Time, report func(processedBlueprints, generatedSchedules int) error) (BackfillResult, error) {
	var candidates []dao.BlueprintRevision
	if err := uc.uow.Do(ctx, func(ctx context.Context, repos Repositories) error {
		var err error
		candidates, err = repos.Blueprints().ListRevisionsForBackfill(ctx)
		return err
	}); err != nil {
		return BackfillResult{}, err
	}

	blueprintSet := make(map[string]struct{}, len(candidates))
	for _, revision := range candidates {
		blueprintSet[revision.ID] = struct{}{}
	}
	blueprintIDs := make([]string, 0, len(blueprintSet))
	for id := range blueprintSet {
		blueprintIDs = append(blueprintIDs, id)
	}
	sort.Strings(blueprintIDs)

	cutoff = cutoff.UTC()
	result := BackfillResult{}
	for _, blueprintID := range blueprintIDs {
		if err := ctx.Err(); err != nil {
			return result, err
		}

		var generated int
		var processed bool
		err := uc.uow.Do(ctx, func(ctx context.Context, repos Repositories) error {
			// Read the current associations before locking Projects. If a Project
			// was deleted since the job's initial scan, the repository omits it and
			// this blueprint can be skipped without repeatedly failing the job.
			before, err := repos.Blueprints().ListRevisionsForBlueprintBackfill(ctx, blueprintID)
			if err != nil {
				return err
			}
			if len(before) == 0 {
				return nil
			}
			beforeProjects := uniqueProjectIDs(before)
			for _, projectID := range beforeProjects {
				if err := repos.Schedules().LockProjectForScheduleMutation(ctx, domain.ProjectID(projectID)); err != nil {
					return err
				}
			}
			if err := repos.Blueprints().LockBlueprintForBackfill(ctx, blueprintID); err != nil {
				return err
			}

			// The Project locks and blueprint lock serialize this run against
			// association edits. A changed lock set is a stable conflict: stop the
			// job, retain earlier per-blueprint commits, and let the next run resume.
			after, err := repos.Blueprints().ListRevisionsForBlueprintBackfill(ctx, blueprintID)
			if err != nil {
				return err
			}
			if !sameStrings(beforeProjects, uniqueProjectIDs(after)) {
				return ErrBackfillProjectSetChanged
			}
			for _, revision := range after {
				count, err := uc.backfillRevision(ctx, repos, actorID, revision, cutoff)
				if err != nil {
					return err
				}
				generated += count
			}
			processed = true
			return nil
		})
		if err != nil {
			return result, err
		}
		if processed {
			result.ProcessedBlueprints++
		}
		result.GeneratedSchedules += generated
		if processed && report != nil {
			if err := report(result.ProcessedBlueprints, result.GeneratedSchedules); err != nil {
				return result, err
			}
		}
	}
	return result, nil
}

func (uc *BackfillUseCase) backfillRevision(ctx context.Context, repos Repositories, actorID string, revision dao.BlueprintRevision, cutoff time.Time) (int, error) {
	location, err := time.LoadLocation(revision.Timezone)
	if err != nil {
		return 0, recurrence.ErrRecurrenceTimezoneInvalid
	}
	localCutoffDate := localCalendarDate(cutoff.In(location))
	start := parseLocalTimestamp(revision.LocalStart)
	effective, err := time.Parse("2006-01-02", revision.EffectiveFrom)
	if err != nil {
		return 0, err
	}
	anchor := parseDateValue(revision.RecurrenceAnchorDate)
	end := parseLocalTimestamp(revision.LocalEnd)
	var ruleEnd *time.Time
	if revision.EndDate != nil {
		value, err := time.Parse("2006-01-02", *revision.EndDate)
		if err != nil {
			return 0, err
		}
		ruleEnd = &value
	}
	rule := recurrence.BlueprintRule{
		Start: start, AnchorDate: anchor, End: end, Timezone: revision.Timezone,
		Frequency: revision.Frequency, Interval: revision.Interval,
		Weekdays: mustParseWeekdays(revision.Weekdays), EndDate: ruleEnd,
	}

	checkpoint, err := repos.Blueprints().GetRevisionBackfillCheckpoint(ctx, revision.RevisionID)
	if err != nil {
		return 0, err
	}
	from := localCalendarDate(start)
	if effective.After(from) {
		from = effective
	}
	if checkpoint != nil {
		date, err := time.Parse("2006-01-02", *checkpoint)
		if err != nil {
			return 0, err
		}
		from = date.AddDate(0, 0, 1)
	}

	// A cutoff is exclusive by instant, not by local date. Include today's
	// definition when its original start is before the cutoff; leave the
	// checkpoint on yesterday when today's start is equal to or after cutoff.
	through := localCutoffDate
	if revision.EffectiveUntil != nil {
		endDate, err := time.Parse("2006-01-02", *revision.EffectiveUntil)
		if err != nil {
			return 0, err
		}
		if endDate.Before(through) {
			through = endDate
		}
	}
	if ruleEnd != nil && ruleEnd.Before(through) {
		through = *ruleEnd
	}
	through, err = lastSafeBackfillDate(rule, from, through, localCutoffDate, cutoff)
	if err != nil {
		return 0, err
	}
	if through.Before(from) {
		return 0, nil
	}

	occurrences, err := recurrence.GenerateBlueprintOccurrences(rule, from, through)
	if err != nil {
		return 0, err
	}
	generated := 0
	for _, occurrence := range occurrences {
		if !occurrence.StartAt.Before(cutoff) {
			continue
		}
		id := uc.ids.Generate()
		row := dao.ScheduleOccurrence{
			ID: &id, BlueprintID: revision.ID, OccurrenceDate: occurrence.Date.Format("2006-01-02"),
			SourceRevisionID: revision.RevisionID, UserID: revision.UserID, ProjectID: revision.ProjectID,
			AssigneeID: revision.AssigneeID, Title: revision.Title, Description: revision.Description,
			Location: revision.Location, Timezone: revision.Timezone,
			StartAt: occurrence.StartAt.Unix(), EndAt: occurrence.EndAt.Unix(),
		}
		inserted, err := repos.Blueprints().BackfillOccurrence(ctx, actorID, row)
		if err != nil {
			return generated, err
		}
		if inserted {
			generated++
		}
	}
	return generated, repos.Blueprints().AdvanceRevisionBackfillCheckpoint(ctx, revision.RevisionID, through.Format("2006-01-02"))
}

func lastSafeBackfillDate(rule recurrence.BlueprintRule, from, through, localCutoffDate time.Time, cutoff time.Time) (time.Time, error) {
	if through.Before(from) || localCutoffDate.Before(from) || through.Before(localCutoffDate) {
		return through, nil
	}
	todays, err := recurrence.GenerateBlueprintOccurrences(rule, localCutoffDate, localCutoffDate)
	if err != nil {
		return time.Time{}, err
	}
	if len(todays) != 0 && !todays[0].StartAt.Before(cutoff) {
		return localCutoffDate.AddDate(0, 0, -1), nil
	}
	return through, nil
}

func uniqueProjectIDs(revisions []dao.BlueprintRevision) []string {
	seen := map[string]struct{}{}
	for _, revision := range revisions {
		if revision.ProjectID != "" {
			seen[revision.ProjectID] = struct{}{}
		}
	}
	out := make([]string, 0, len(seen))
	for projectID := range seen {
		out = append(out, projectID)
	}
	sort.Strings(out)
	return out
}

func sameStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

var _ ScheduleBackfill = (*BackfillUseCase)(nil)
