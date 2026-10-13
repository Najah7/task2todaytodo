package usecase

import (
	"context"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/allocation/dao"
	"github.com/Najah7/task2todaytodo/internal/application/allocation/domain"
	"github.com/Najah7/task2todaytodo/internal/application/shared/calendar"
)

type UpdateOccurrenceUseCase struct {
	repo    Repository
	uow     UnitOfWork
	issueID func() string
	now     func() time.Time
}

func NewUpdateOccurrenceUseCase(repo Repository, uow UnitOfWork, issueID func() string, now func() time.Time) *UpdateOccurrenceUseCase {
	return &UpdateOccurrenceUseCase{repo: repo, uow: uow, issueID: issueID, now: now}
}

func (uc *UpdateOccurrenceUseCase) Execute(ctx context.Context, userID, ruleID string, occurrenceDate time.Time, expected int32, projectID string, startAt, endAt time.Time, scope string, ruleRevision int32) (dao.Occurrence, error) {
	if userID == "" || ruleID == "" || expected < 0 || projectID == "" || !endAt.After(startAt) || (scope != "current" && scope != "following") || uc.issueID == nil {
		return dao.Occurrence{}, ErrInvalidRequest
	}
	occurrenceDate = calendar.DateOnly(occurrenceDate)
	now := uc.now().UTC()
	var result dao.Occurrence
	err := uc.uow.Do(ctx, func(ctx context.Context, repo Repository) error {
		current, _, err := resolveOccurrence(ctx, repo, userID, ruleID, occurrenceDate)
		if err != nil {
			return err
		}
		if err := repo.LockProjectsForMutation(ctx, userID, []string{current.ProjectID, projectID}); err != nil {
			return err
		}
		if err := repo.LockUser(ctx, userID); err != nil {
			return err
		}
		current, virtual, err := resolveOccurrence(ctx, repo, userID, ruleID, occurrenceDate)
		if err != nil {
			return err
		}
		if virtual {
			if expected != 0 {
				return ErrPreconditionFailed
			}
			current, err = repo.MaterializeOccurrence(ctx, userID, domain.Occurrence{ID: uc.issueID(), RuleID: current.RuleID, SourceRevisionID: current.SourceRevisionID, ProjectID: current.ProjectID, OccurrenceDate: current.OccurrenceDate, StartAt: current.StartAt, EndAt: current.EndAt, Timezone: current.Timezone}, now)
			if err != nil {
				return err
			}
		} else if current.Revision != expected {
			return ErrPreconditionFailed
		}
		persistedRevision := current.Revision
		rules, err := repo.ListRules(ctx, userID)
		if err != nil {
			return err
		}
		var oldRule dao.Rule
		ruleFound := false
		for _, rule := range rules {
			if rule.ID == ruleID {
				oldRule = rule
				ruleFound = true
				break
			}
		}
		if !ruleFound && scope == "current" {
			// Following-delete stops future cadence by setting DeletedAt. Earlier
			// retained frames remain individually editable, so resolve their latest
			// Rule metadata from the historical projection revisions.
			revisions, err := repo.ListRuleRevisionsForProjection(ctx, userID)
			if err != nil {
				return err
			}
			for _, rule := range revisions {
				if rule.ID == ruleID && (!ruleFound || rule.Revision > oldRule.Revision) {
					oldRule, ruleFound = rule, true
				}
			}
		}
		if !ruleFound {
			return ErrNotFound
		}
		if scope == "following" {
			if oldRule.DeletedAt != nil || oldRule.Revision != ruleRevision {
				return ErrPreconditionFailed
			}
		}
		projectStatus, err := repo.GetProjectStatus(ctx, userID, projectID)
		if err != nil {
			return err
		}
		materialized := make(map[string]dao.Allocation)
		if scope == "following" {
			rows, err := repo.ListAllocations(ctx, userID, now, now.AddDate(0, 0, 400))
			if err != nil {
				return err
			}
			for _, row := range rows {
				if row.RuleID == ruleID && !row.IsException && !row.StartAt.Before(now) {
					materialized[occurrenceOriginKey(row.RuleID, row.OccurrenceDate)] = row
				}
			}
		}
		location, err := time.LoadLocation(current.Timezone)
		if err != nil {
			return ErrInvalidRequest
		}
		localStart, localEnd := startAt.In(location), endAt.In(location)
		startDate, endDate := calendar.DateOnly(localStart), calendar.DateOnly(localEnd)
		endOffset := int(endDate.Sub(startDate).Hours() / 24)
		if endOffset < 0 || endOffset > 1 {
			return ErrInvalidRequest
		}
		if _, err := domain.NewSlot(int(localStart.Weekday()), localStart.Format("15:04"), localEnd.Format("15:04"), endOffset, current.Timezone, "once"); err != nil {
			return ErrInvalidRequest
		}
		candidate := domain.Occurrence{ID: current.ID, RuleID: ruleID, SourceRevisionID: current.SourceRevisionID, ProjectID: projectID, OccurrenceDate: occurrenceDate, StartAt: startAt.UTC(), EndAt: endAt.UTC(), Timezone: current.Timezone}
		if scope == "following" {
			weekly, err := domain.NewSlot(int(localStart.Weekday()), localStart.Format("15:04"), localEnd.Format("15:04"), endOffset, current.Timezone, "weekly")
			if err != nil {
				return ErrInvalidRequest
			}
			effective := occurrenceDate
			weeklyStart := calendar.DateOnly(candidate.StartAt.In(location)).AddDate(0, 0, 1)
			through := effective.AddDate(0, 0, 400)
			generated, err := calendar.GenerateOccurrences(recurrenceFor(ruleID, "", projectID, weekly, weeklyStart), weeklyStart, through)
			if err != nil {
				return ErrInvalidRequest
			}
			exceptionOrigins, err := repo.ListExceptionOrigins(ctx, userID, effective, through)
			if err != nil {
				return err
			}
			following := make([]domain.Occurrence, 0, len(generated))
			for _, item := range generated {
				if item.Date.Equal(occurrenceDate) || item.StartAt.Before(now) || exceptionOrigins[occurrenceOriginKey(ruleID, item.Date)] {
					continue
				}
				if projectStatus != "done" {
					following = append(following, domain.Occurrence{ID: uc.issueID(), RuleID: ruleID, SourceRevisionID: oldRule.SourceRevisionID, ProjectID: projectID, OccurrenceDate: item.Date, StartAt: item.StartAt, EndAt: item.EndAt, Timezone: current.Timezone})
				}
			}
			if err := repo.DeleteFutureOrdinaryOriginsAfter(ctx, userID, ruleID, occurrenceDate, now); err != nil {
				return err
			}
			if projectStatus != "done" {
				if err := validateAvailability(ctx, repo, userID, append([]domain.Occurrence{candidate}, following...), ruleID, now); err != nil {
					return err
				}
			}
			if err := repo.SupersedeFutureRevisions(ctx, ruleID, now); err != nil {
				return err
			}
			candidate.SourceRevisionID = uc.issueID()
			if err := repo.CreateRuleRevision(ctx, userID, ruleID, projectID, candidate.SourceRevisionID, ruleRevision+1, effective, weeklyStart, now, weekly); err != nil {
				return err
			}
			if oldRule.PausedReason == "project_completed" && oldRule.ProjectID != projectID {
				if err := repo.ClosePauseInterval(ctx, oldRule.ProjectID, ruleID, now); err != nil {
					return err
				}
			}
			if projectStatus == "done" && oldRule.PausedReason != "project_completed" {
				if err := repo.SetRulePause(ctx, userID, ruleID, "project_completed", now); err != nil {
					return err
				}
				if err := repo.StartPauseInterval(ctx, projectID, ruleID, now); err != nil {
					return err
				}
			} else if projectStatus != "done" && (oldRule.PausedReason == "allocation_conflict" || oldRule.PausedReason == "outside_active_time" || oldRule.PausedReason == "project_completed") {
				if err := repo.SetRulePause(ctx, userID, ruleID, "", now); err != nil {
					return err
				}
			}
			for _, occurrence := range following {
				key := occurrenceOriginKey(ruleID, occurrence.OccurrenceDate)
				previous, exists := materialized[key]
				if !exists {
					continue
				}
				occurrence.ID = previous.ID
				occurrence.SourceRevisionID = candidate.SourceRevisionID
				if err := repo.RefreshOccurrence(ctx, userID, occurrence, now); err != nil {
					return err
				}
			}
		} else {
			// A current-only edit leaves the rest of this Rule's cadence intact,
			// so check its other virtual occurrences too. The exact origin being
			// edited is excluded by validateRecurringRules and the concrete row
			// lookup independently.
			if err := validateAvailability(ctx, repo, userID, []domain.Occurrence{candidate}, "", now); err != nil {
				return err
			}
		}
		if err := repo.SaveOccurrenceException(ctx, userID, candidate, persistedRevision, false); err != nil {
			return err
		}
		stored, err := repo.GetOccurrence(ctx, ruleID, userID, occurrenceDate)
		if err != nil {
			return err
		}
		result = occurrenceFromAllocation(stored)
		return nil
	})
	return result, err
}

func resolveOccurrence(ctx context.Context, repo Repository, userID, ruleID string, date time.Time) (dao.Allocation, bool, error) {
	current, err := repo.ReadOccurrence(ctx, ruleID, userID, date)
	if err == nil {
		return current, false, nil
	}
	if err != ErrNotFound {
		return dao.Allocation{}, false, err
	}
	rules, err := repo.ListRuleRevisionsForProjection(ctx, userID)
	if err != nil {
		return dao.Allocation{}, false, err
	}
	for _, rule := range rules {
		if rule.ID != ruleID {
			continue
		}
		projected, err := projectedRuleOccurrences(ctx, repo, userID, rule, date.AddDate(0, 0, -1), date.AddDate(0, 0, 1))
		if err != nil {
			return dao.Allocation{}, false, err
		}
		for _, occurrence := range projected {
			if !occurrence.OccurrenceDate.Equal(date) {
				continue
			}
			return occurrence, true, nil
		}
	}
	return dao.Allocation{}, false, ErrNotFound
}
