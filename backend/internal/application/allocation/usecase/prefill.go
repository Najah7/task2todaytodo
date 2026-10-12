package usecase

import (
	"context"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/allocation/domain"
	"github.com/Najah7/task2todaytodo/internal/application/shared/calendar"
)

// PrefillAllocationsExecutor materializes missing calendar projections for the
// current and next two user-local weeks. Each user commits independently so a
// retry safely resumes at the first user whose work was not committed.
type PrefillAllocationsExecutor struct {
	repo    Repository
	uow     UnitOfWork
	issueID func() string
}

func NewPrefillAllocationsExecutor(repo Repository, uow UnitOfWork, issueID func() string) *PrefillAllocationsExecutor {
	return &PrefillAllocationsExecutor{repo: repo, uow: uow, issueID: issueID}
}

func (e *PrefillAllocationsExecutor) Execute(ctx context.Context, _ string, cutoff time.Time, report func(int, int) error) (int, int, error) {
	if e.issueID == nil {
		return 0, 0, ErrInvalidRequest
	}
	users, err := e.repo.ListUserIDs(ctx)
	if err != nil {
		return 0, 0, err
	}
	processedRules, generated := 0, 0
	for _, userID := range users {
		if err := ctx.Err(); err != nil {
			return processedRules, generated, err
		}
		processed, created, err := e.prefillUser(ctx, userID, cutoff.UTC())
		if err != nil {
			return processedRules, generated, err
		}
		processedRules += processed
		generated += created
		if report != nil {
			if err := report(processedRules, generated); err != nil {
				return processedRules, generated, err
			}
		}
	}
	return processedRules, generated, nil
}

func (e *PrefillAllocationsExecutor) prefillUser(ctx context.Context, userID string, cutoff time.Time) (processedRules, generated int, err error) {
	// Discover lock keys before entering the transaction; all data is reread
	// after acquiring Project locks followed by the user's allocation lock.
	rules, err := e.repo.ListRuleRevisionsForProjection(ctx, userID)
	if err != nil {
		return 0, 0, err
	}
	projectIDs := make([]string, 0, len(rules))
	for _, rule := range rules {
		projectIDs = append(projectIDs, rule.ProjectID)
	}
	err = e.uow.Do(ctx, func(ctx context.Context, repo Repository) error {
		if len(projectIDs) != 0 {
			if err := repo.LockProjectsForMutation(ctx, userID, projectIDs); err != nil {
				return err
			}
		}
		if err := repo.LockUser(ctx, userID); err != nil {
			return err
		}
		profile, err := repo.GetProfile(ctx, userID)
		if err != nil {
			return err
		}
		location, err := time.LoadLocation(profile.Timezone)
		if err != nil {
			return ErrInvalidRequest
		}
		localCutoff := cutoff.In(location)
		weekStartDate := calendar.DateOnly(localCutoff)
		weekStartDate = weekStartDate.AddDate(0, 0, -((int(weekStartDate.Weekday()) + 6) % 7))
		weekStart, ok := calendar.ResolveForward(weekStartDate, time.Time{}, 0, location)
		if !ok {
			return ErrInvalidRequest
		}
		weekEndDate := weekStartDate.AddDate(0, 0, 21)
		weekEnd, ok := calendar.ResolveForward(weekEndDate, time.Time{}, 0, location)
		if !ok {
			return ErrInvalidRequest
		}
		rules, err := repo.ListRuleRevisionsForProjection(ctx, userID)
		if err != nil {
			return err
		}
		frames, err := repo.ListAllocations(ctx, userID, weekStart, weekEnd)
		if err != nil {
			return err
		}
		existing := make(map[string]bool, len(frames))
		for _, frame := range frames {
			existing[occurrenceOriginKey(frame.RuleID, frame.OccurrenceDate)] = true
		}
		seenRules := make(map[string]bool)
		for _, rule := range rules {
			if rule.ProjectID == "" || rule.ID == "" {
				continue
			}
			from := weekStartDate.AddDate(0, 0, -2)
			through := weekEndDate.AddDate(0, 0, 2)
			occurrences, err := projectedRuleOccurrences(ctx, repo, userID, rule, from, through)
			if err != nil {
				return err
			}
			eligible := false
			for _, occurrence := range occurrences {
				if occurrence.StartAt.Before(weekStart) || !occurrence.StartAt.Before(weekEnd) {
					continue
				}
				eligible = true
				key := occurrenceOriginKey(occurrence.RuleID, occurrence.OccurrenceDate)
				if existing[key] {
					continue
				}
				if err := repo.RefreshOccurrence(ctx, userID, domain.Occurrence{
					ID: e.issueID(), RuleID: occurrence.RuleID, SourceRevisionID: occurrence.SourceRevisionID,
					ProjectID: occurrence.ProjectID, OccurrenceDate: occurrence.OccurrenceDate,
					StartAt: occurrence.StartAt, EndAt: occurrence.EndAt, Timezone: occurrence.Timezone,
				}, cutoff); err != nil {
					return err
				}
				existing[key] = true
				generated++
			}
			if eligible && !seenRules[rule.ID] {
				processedRules++
				seenRules[rule.ID] = true
			}
		}
		return nil
	})
	return processedRules, generated, err
}

var _ interface {
	Execute(context.Context, string, time.Time, func(int, int) error) (int, int, error)
} = (*PrefillAllocationsExecutor)(nil)
