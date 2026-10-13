package usecase

import (
	"context"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/allocation/dao"
	"github.com/Najah7/task2todaytodo/internal/application/allocation/domain"
	"github.com/Najah7/task2todaytodo/internal/application/shared/calendar"
)

func validateRules(items []dao.Rule) error {
	ruleSeen := map[string]bool{}
	for _, rule := range items {
		if rule.ProjectID == "" {
			return ErrInvalidRequest
		}
		if rule.ID != "" && ruleSeen[rule.ID] {
			return ErrInvalidRequest
		}
		if rule.ID != "" {
			ruleSeen[rule.ID] = true
		}
		if _, err := domain.NewSlot(rule.Weekday, rule.StartTime, rule.EndTime, rule.EndDayOffset, rule.Timezone, "weekly"); err != nil {
			return ErrInvalidRequest
		}
		if (rule.ID == "") != (rule.Revision == 0) {
			return ErrInvalidRequest
		}
	}
	return nil
}

func replaceRules(ctx context.Context, repo Repository, userID string, requestedRules, currentRules []dao.Rule, projectStatuses map[string]string, now time.Time, issueID func() string) error {
	oldRules := make(map[string]dao.Rule)
	for _, rule := range currentRules {
		oldRules[rule.ID] = rule
	}
	materialized, err := repo.ListAllocations(ctx, userID, now, now.AddDate(0, 0, 400))
	if err != nil {
		return err
	}
	materializedByOrigin := make(map[string]dao.Allocation, len(materialized))
	for _, row := range materialized {
		if !row.IsException && !row.StartAt.Before(now) {
			materializedByOrigin[occurrenceOriginKey(row.RuleID, row.OccurrenceDate)] = row
		}
	}
	newRules := make(map[string]bool)
	candidates := make([]domain.Occurrence, 0)
	projectDone := make(map[string]bool, len(projectStatuses))
	for id, status := range projectStatuses {
		projectDone[id] = status == "done"
	}
	for _, requested := range requestedRules {
		projectID := requested.ProjectID
		ruleID, nextRevision := requested.ID, requested.Revision+1
		var old dao.Rule
		if ruleID == "" {
			ruleID, nextRevision = issueID(), 1
			if err := repo.CreateRule(ctx, userID, ruleID, projectID); err != nil {
				return err
			}
		} else {
			var exists bool
			old, exists = oldRules[ruleID]
			if !exists || old.Revision != requested.Revision {
				return ErrPreconditionFailed
			}
			if old.ProjectID != projectID {
				return ErrInvalidRequest
			}
			if sameWeeklyRule(old, requested) {
				if projectDone[projectID] && old.PausedReason != "project_completed" {
					if err := repo.SetRulePause(ctx, userID, ruleID, "project_completed", now); err != nil {
						return err
					}
					if err := repo.StartPauseInterval(ctx, projectID, ruleID, now); err != nil {
						return err
					}
				}
				newRules[ruleID] = true
				continue
			}
			if err := repo.DeleteFutureRows(ctx, ruleID, now, userID, false); err != nil {
				return err
			}
			if err := repo.SupersedeFutureRevisions(ctx, ruleID, now); err != nil {
				return err
			}
		}
		slot, err := domain.NewSlot(requested.Weekday, requested.StartTime, requested.EndTime, requested.EndDayOffset, requested.Timezone, "weekly")
		if err != nil {
			return ErrInvalidRequest
		}
		location, err := time.LoadLocation(slot.Timezone)
		if err != nil {
			return ErrInvalidRequest
		}
		effectiveFrom := calendar.DateOnly(now.In(location))
		sourceID := issueID()
		if err := repo.CreateRuleRevision(ctx, userID, ruleID, projectID, sourceID, nextRevision, effectiveFrom, effectiveFrom, now, slot); err != nil {
			return err
		}
		if projectDone[projectID] {
			if err := repo.SetRulePause(ctx, userID, ruleID, "project_completed", now); err != nil {
				return err
			}
			if old.PausedReason != "project_completed" {
				if err := repo.StartPauseInterval(ctx, projectID, ruleID, now); err != nil {
					return err
				}
			}
		} else if old.PausedReason == "allocation_conflict" || old.PausedReason == "outside_active_time" {
			if err := repo.SetRulePause(ctx, userID, ruleID, "", now); err != nil {
				return err
			}
		}
		newRules[ruleID] = true
		rule := dao.Rule{ID: ruleID, ProjectID: projectID, Revision: nextRevision, SourceRevisionID: sourceID, Weekday: slot.Weekday, StartTime: slot.Start.Format("15:04"), EndTime: slot.End.Format("15:04"), EndDayOffset: slot.EndDayOffset, Timezone: slot.Timezone, Frequency: "weekly", EffectiveFrom: effectiveFrom, EffectiveAt: now, RecurrenceStartDate: effectiveFrom}
		generated, err := projectedRuleOccurrences(ctx, repo, userID, rule, effectiveFrom, effectiveFrom.AddDate(0, 0, 400))
		if err != nil {
			return err
		}
		if !projectDone[projectID] {
			for _, occurrence := range generated {
				candidates = append(candidates, domain.Occurrence{RuleID: occurrence.RuleID, SourceRevisionID: sourceID, ProjectID: occurrence.ProjectID, OccurrenceDate: occurrence.OccurrenceDate, StartAt: occurrence.StartAt, EndAt: occurrence.EndAt, Timezone: occurrence.Timezone})
			}
		}
	}
	for ruleID := range oldRules {
		if newRules[ruleID] {
			continue
		}
		if err := repo.DeleteFutureRows(ctx, ruleID, now, userID, true); err != nil {
			return err
		}
		if err := repo.StopRule(ctx, userID, ruleID, now); err != nil {
			return err
		}
	}
	for i, candidate := range candidates {
		for _, other := range candidates[:i] {
			if calendar.Overlaps(candidate.StartAt, candidate.EndAt, other.StartAt, other.EndAt) {
				return conflict("allocation_conflict", other)
			}
		}
		if err := validateAvailability(ctx, repo, userID, []domain.Occurrence{candidate}, candidate.RuleID, now); err != nil {
			return err
		}
	}
	for _, candidate := range candidates {
		old, exists := materializedByOrigin[occurrenceOriginKey(candidate.RuleID, candidate.OccurrenceDate)]
		if !exists {
			continue
		}
		candidate.ID = old.ID
		if err := repo.RefreshOccurrence(ctx, userID, candidate, now); err != nil {
			return err
		}
	}
	return nil
}

func sameWeeklyRule(old, requested dao.Rule) bool {
	return old.Frequency == "weekly" && old.Weekday == requested.Weekday &&
		old.StartTime == requested.StartTime && old.EndTime == requested.EndTime &&
		old.EndDayOffset == requested.EndDayOffset && old.Timezone == requested.Timezone
}
