package usecase

import (
	"context"
	"sort"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/allocation/dao"
	"github.com/Najah7/task2todaytodo/internal/application/allocation/domain"
	"github.com/Najah7/task2todaytodo/internal/application/shared/calendar"
)

func validateAvailability(ctx context.Context, repo Repository, userID string, candidates []domain.Occurrence, excludeRule string, now time.Time) error {
	if len(candidates) == 0 {
		return ErrInvalidRequest
	}
	for i, candidate := range candidates {
		for _, other := range candidates[:i] {
			if calendar.Overlaps(candidate.StartAt, candidate.EndAt, other.StartAt, other.EndAt) {
				return conflict("allocation_conflict", other)
			}
		}
		conflictRow, err := repo.FindAllocationConflict(ctx, userID, candidate.StartAt, candidate.EndAt, candidate.RuleID, candidate.OccurrenceDate)
		if err == nil {
			return conflictFromRow("allocation_conflict", conflictRow)
		}
		if err != ErrNoConflict {
			return err
		}
	}
	profile, err := repo.GetProfile(ctx, userID)
	if err != nil {
		return err
	}
	location, err := time.LoadLocation(profile.Timezone)
	if err != nil {
		return ErrInvalidRequest
	}
	minDate, maxDate := candidates[0].StartAt.In(location).AddDate(0, 0, -2), candidates[0].EndAt.In(location).AddDate(0, 0, 2)
	for _, candidate := range candidates[1:] {
		if candidate.StartAt.In(location).Before(minDate) {
			minDate = candidate.StartAt.In(location).AddDate(0, 0, -2)
		}
		if candidate.EndAt.In(location).After(maxDate) {
			maxDate = candidate.EndAt.In(location).AddDate(0, 0, 2)
		}
	}
	dateFrom, dateThrough := calendar.DateOnly(minDate), calendar.DateOnly(maxDate)
	exceptions, err := repo.ListActiveTimeExceptions(ctx, userID, dateFrom, dateThrough)
	if err != nil {
		return err
	}
	exceptionByDate := make(map[string]dao.ActiveTimeException, len(exceptions))
	for _, exception := range exceptions {
		exceptionByDate[exception.LocalDate.Format("2006-01-02")] = exception
	}
	activeStartDefault, _ := time.Parse("15:04", profile.ActiveStartTime)
	activeEndTimeDefault, _ := time.Parse("15:04", profile.ActiveEndTime)
	activeWindow := make(map[string]domain.Interval)
	for date := dateFrom; !date.After(dateThrough); date = date.AddDate(0, 0, 1) {
		wake, activeEndTime, zone := activeStartDefault, activeEndTimeDefault, profile.Timezone
		var interval domain.Interval
		if exception, ok := exceptionByDate[date.Format("2006-01-02")]; ok {
			interval, err = domain.ActiveWindowFromLocalPair(date, exception.ActiveStartLocal, exception.ActiveEndLocal, exception.Timezone)
		} else {
			interval, err = domain.ActiveWindow(date, wake, activeEndTime, zone)
		}
		if err != nil {
			return err
		}
		activeWindow[date.Format("2006-01-02")] = interval
	}
	for _, candidate := range candidates {
		if !withinActiveTime(candidate.StartAt, candidate.EndAt, activeWindow) {
			return conflict("outside_active_time", candidate)
		}
	}
	if err := validateRecurringRules(ctx, repo, userID, candidates, excludeRule, now); err != nil {
		return err
	}
	return nil
}

func withinActiveTime(start, end time.Time, windows map[string]domain.Interval) bool {
	if !end.After(start) {
		return false
	}
	intervals := make([]domain.Interval, 0, len(windows))
	for _, window := range windows {
		if window.End.After(start) && window.Start.Before(end) {
			intervals = append(intervals, window)
		}
	}
	sort.Slice(intervals, func(i, j int) bool { return intervals[i].Start.Before(intervals[j].Start) })
	cursor := start
	for _, window := range intervals {
		if window.Start.After(cursor) {
			return false
		}
		if window.End.After(cursor) {
			cursor = window.End
		}
		if !cursor.Before(end) {
			return true
		}
	}
	return false
}

func validateRecurringRules(ctx context.Context, repo Repository, userID string, candidates []domain.Occurrence, excludeRule string, now time.Time) error {
	rules, err := repo.ListRuleRevisionsForProjection(ctx, userID)
	if err != nil {
		return err
	}
	through := candidates[0].OccurrenceDate.AddDate(0, 0, 400)
	for _, candidate := range candidates {
		if candidate.OccurrenceDate.AddDate(0, 0, 400).After(through) {
			through = candidate.OccurrenceDate.AddDate(0, 0, 400)
		}
	}
	from := candidates[0].OccurrenceDate
	for _, rule := range rules {
		if rule.ID == excludeRule || rule.PausedReason != "" || (rule.StoppedAfter != nil && rule.StoppedAfter.Before(from)) {
			continue
		}
		occurrences, err := projectedRuleOccurrences(ctx, repo, userID, rule, from.AddDate(0, 0, -1), through)
		if err != nil {
			return err
		}
		for _, other := range occurrences {
			for _, candidate := range candidates {
				if candidate.RuleID == rule.ID && candidate.OccurrenceDate.Format("2006-01-02") == other.OccurrenceDate.Format("2006-01-02") {
					continue
				}
				if candidate.StartAt.Before(now) && candidate.EndAt.Before(now) {
					continue
				}
				if calendar.Overlaps(candidate.StartAt, candidate.EndAt, other.StartAt, other.EndAt) {
					return ConflictError{Kind: "allocation_conflict", RuleID: rule.ID, OccurrenceDate: other.OccurrenceDate, ProjectID: rule.ProjectID, StartAt: other.StartAt, EndAt: other.EndAt}
				}
			}
		}
	}
	return nil
}

func conflict(kind string, occurrence domain.Occurrence) error {
	return ConflictError{Kind: kind, RuleID: occurrence.RuleID, OccurrenceDate: occurrence.OccurrenceDate, ProjectID: occurrence.ProjectID, StartAt: occurrence.StartAt, EndAt: occurrence.EndAt}
}

func conflictFromRow(kind string, allocation dao.Allocation) error {
	return ConflictError{Kind: kind, RuleID: allocation.RuleID, OccurrenceDate: allocation.OccurrenceDate, ProjectID: allocation.ProjectID, StartAt: allocation.StartAt, EndAt: allocation.EndAt}
}
