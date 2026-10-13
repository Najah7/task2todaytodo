package usecase

import (
	"context"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/allocation/dao"
	"github.com/Najah7/task2todaytodo/internal/application/allocation/domain"
	"github.com/Najah7/task2todaytodo/internal/application/shared/calendar"
)

// ProfileAvailabilityValidator is bound to the same SQL transaction as the
// auth write. It serializes by user before checking both persisted allocations
// and future rule occurrences against the proposed active_time window.
type ProfileAvailabilityValidator struct {
	repo Repository
	now  func() time.Time
}

func NewProfileAvailabilityValidator(repo Repository, now func() time.Time) *ProfileAvailabilityValidator {
	if now == nil {
		now = time.Now
	}
	return &ProfileAvailabilityValidator{repo: repo, now: now}
}

func (v *ProfileAvailabilityValidator) LockUser(ctx context.Context, userID string) error {
	return v.repo.LockUser(ctx, userID)
}

func (v *ProfileAvailabilityValidator) ValidateUserProfile(ctx context.Context, userID, timezone, activeStartTime, activeEndTime string) error {
	if userID == "" {
		return ErrInvalidRequest
	}
	if err := v.repo.LockUser(ctx, userID); err != nil {
		return err
	}
	location, err := time.LoadLocation(timezone)
	if err != nil {
		return ErrInvalidRequest
	}
	now := v.now().UTC()
	from := calendar.DateOnly(now.In(location))
	through := from.AddDate(0, 0, 400)
	rules, err := v.repo.ListRuleRevisionsForProjection(ctx, userID)
	if err != nil {
		return err
	}
	exceptions, err := v.repo.ListActiveTimeExceptions(ctx, userID, from.AddDate(0, 0, -2), through.AddDate(0, 0, 2))
	if err != nil {
		return err
	}
	byDate := make(map[string]dao.ActiveTimeException, len(exceptions))
	for _, item := range exceptions {
		byDate[item.LocalDate.Format("2006-01-02")] = item
	}
	activeWindow := make(map[string]domain.Interval, 406)
	for date := from.AddDate(0, 0, -2); !date.After(through.AddDate(0, 0, 2)); date = date.AddDate(0, 0, 1) {
		wake, bed, zone := mustWall(activeStartTime), mustWall(activeEndTime), timezone
		var interval domain.Interval
		if item, ok := byDate[date.Format("2006-01-02")]; ok {
			interval, err = domain.ActiveWindowFromLocalPair(date, item.ActiveStartLocal, item.ActiveEndLocal, item.Timezone)
		} else {
			interval, err = domain.ActiveWindow(date, wake, bed, zone)
		}
		if err != nil {
			return ErrInvalidRequest
		}
		activeWindow[date.Format("2006-01-02")] = interval
	}
	for date := from.AddDate(0, 0, -1); !date.After(through.AddDate(0, 0, 1)); date = date.AddDate(0, 0, 1) {
		previous, current := activeWindow[date.AddDate(0, 0, -1).Format("2006-01-02")], activeWindow[date.Format("2006-01-02")]
		if current.Start.Before(previous.End) {
			return ErrInvalidRequest
		}
	}
	for _, rule := range rules {
		occurrences, err := projectedRuleOccurrences(ctx, v.repo, userID, rule, from.AddDate(0, 0, -1), through)
		if err != nil {
			return err
		}
		for _, occ := range occurrences {
			candidate := domain.Occurrence{RuleID: rule.ID, ProjectID: rule.ProjectID, OccurrenceDate: occ.OccurrenceDate, StartAt: occ.StartAt, EndAt: occ.EndAt, Timezone: rule.Timezone}
			if err := validateWithinActiveTime(candidate, activeWindow); err != nil {
				return err
			}
		}
	}
	start, ok := calendar.ResolveForward(from.AddDate(0, 0, -2), time.Time{}, 0, location)
	if !ok {
		return ErrInvalidRequest
	}
	end, ok := calendar.ResolveForward(through.AddDate(0, 0, 3), time.Time{}, 0, location)
	if !ok {
		return ErrInvalidRequest
	}
	allocations, err := v.repo.ListAllocations(ctx, userID, start, end)
	if err != nil {
		return err
	}
	for _, allocation := range allocations {
		candidate := domain.Occurrence{RuleID: allocation.RuleID, ProjectID: allocation.ProjectID, OccurrenceDate: allocation.OccurrenceDate, StartAt: allocation.StartAt, EndAt: allocation.EndAt, Timezone: allocation.Timezone}
		if err := validateWithinActiveTime(candidate, activeWindow); err != nil {
			return err
		}
	}
	return nil
}

func validateWithinActiveTime(candidate domain.Occurrence, activeWindow map[string]domain.Interval) error {
	if !withinActiveTime(candidate.StartAt, candidate.EndAt, activeWindow) {
		return conflict("outside_active_time", candidate)
	}
	return nil
}

func mustWall(value string) time.Time { parsed, _ := time.Parse("15:04", value); return parsed }
