package usecase

import (
	"context"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/allocation/dao"
	"github.com/Najah7/task2todaytodo/internal/application/allocation/domain"
	"github.com/Najah7/task2todaytodo/internal/application/shared/calendar"
)

type PutActiveTimeExceptionUseCase struct {
	repo Repository
	uow  UnitOfWork
	now  func() time.Time
}

func NewPutActiveTimeExceptionUseCase(repo Repository, uow UnitOfWork, now func() time.Time) *PutActiveTimeExceptionUseCase {
	return &PutActiveTimeExceptionUseCase{repo: repo, uow: uow, now: now}
}

func (uc *PutActiveTimeExceptionUseCase) Execute(ctx context.Context, userID string, date time.Time, expected int32, activeStartLocal, activeEndTimeLocal time.Time, timezone string) (dao.ActiveTimeException, error) {
	if userID == "" || expected < 0 || timezone == "" {
		return dao.ActiveTimeException{}, ErrInvalidRequest
	}
	date = calendar.DateOnly(date)
	if calendar.DateOnly(activeStartLocal).Format("2006-01-02") != date.Format("2006-01-02") || calendar.DateOnly(activeEndTimeLocal).Before(date) || calendar.DateOnly(activeEndTimeLocal).After(date.AddDate(0, 0, 1)) {
		return dao.ActiveTimeException{}, ErrInvalidRequest
	}
	exception := domain.ActiveTimeException{LocalDate: date, ActiveStartLocal: activeStartLocal, ActiveEndLocal: activeEndTimeLocal, Timezone: timezone}
	var result dao.ActiveTimeException
	err := uc.uow.Do(ctx, func(ctx context.Context, repo Repository) error {
		if err := repo.LockUser(ctx, userID); err != nil {
			return err
		}
		if err := validateActiveTimeChange(ctx, repo, userID, exception, date, uc.now()); err != nil {
			return err
		}
		stored, err := repo.SaveActiveTimeException(ctx, userID, expected, exception)
		if err != nil {
			return err
		}
		result = stored
		return nil
	})
	return result, err
}

type DeleteActiveTimeExceptionUseCase struct {
	repo Repository
	uow  UnitOfWork
	now  func() time.Time
}

func NewDeleteActiveTimeExceptionUseCase(repo Repository, uow UnitOfWork, now func() time.Time) *DeleteActiveTimeExceptionUseCase {
	if now == nil {
		now = time.Now
	}
	return &DeleteActiveTimeExceptionUseCase{repo: repo, uow: uow, now: now}
}

func (uc *DeleteActiveTimeExceptionUseCase) Execute(ctx context.Context, userID string, date time.Time, expected int32) error {
	if userID == "" || expected <= 0 {
		return ErrInvalidRequest
	}
	date = calendar.DateOnly(date)
	return uc.uow.Do(ctx, func(ctx context.Context, repo Repository) error {
		if err := repo.LockUser(ctx, userID); err != nil {
			return err
		}
		reset := domain.ActiveTimeException{LocalDate: date}
		if err := validateActiveTimeChange(ctx, repo, userID, reset, date, uc.now()); err != nil {
			return err
		}
		return repo.DeleteActiveTimeException(ctx, userID, date, expected)
	})
}

func validateActiveTimeChange(ctx context.Context, repo Repository, userID string, replacement domain.ActiveTimeException, changedDate time.Time, now time.Time) error {
	profile, err := repo.GetProfile(ctx, userID)
	if err != nil {
		return err
	}
	location, err := time.LoadLocation(profile.Timezone)
	if err != nil {
		return ErrInvalidRequest
	}
	fromDate, throughDate := changedDate.AddDate(0, 0, -3), changedDate.AddDate(0, 0, 3)
	windowStart, windowEnd := fromDate.AddDate(0, 0, -1), throughDate.AddDate(0, 0, 1)
	exceptions, err := repo.ListActiveTimeExceptions(ctx, userID, windowStart, windowEnd)
	if err != nil {
		return err
	}
	byDate := make(map[string]dao.ActiveTimeException, len(exceptions)+1)
	for _, item := range exceptions {
		byDate[item.LocalDate.Format("2006-01-02")] = item
	}
	if replacement.Timezone == "" {
		delete(byDate, changedDate.Format("2006-01-02"))
	} else {
		byDate[changedDate.Format("2006-01-02")] = dao.ActiveTimeException{LocalDate: changedDate, ActiveStartLocal: replacement.ActiveStartLocal, ActiveEndLocal: replacement.ActiveEndLocal, Timezone: replacement.Timezone, HasException: true}
	}
	activeStartDefault, _ := time.Parse("15:04", profile.ActiveStartTime)
	activeEndDefault, _ := time.Parse("15:04", profile.ActiveEndTime)
	var prior domain.Interval
	first := true
	activeWindowByDate := make(map[string]domain.Interval, 407)
	for date := windowStart; !date.After(windowEnd); date = date.AddDate(0, 0, 1) {
		wake, bed, zone := activeStartDefault, activeEndDefault, profile.Timezone
		var activeWindow domain.Interval
		if item, ok := byDate[date.Format("2006-01-02")]; ok {
			activeWindow, err = domain.ActiveWindowFromLocalPair(date, item.ActiveStartLocal, item.ActiveEndLocal, item.Timezone)
		} else {
			activeWindow, err = domain.ActiveWindow(date, wake, bed, zone)
		}
		if err != nil {
			return ErrInvalidRequest
		}
		activeWindowByDate[date.Format("2006-01-02")] = activeWindow
		if !first && activeWindow.Start.Before(prior.End) {
			return ErrInvalidRequest
		}
		first = false
		prior = activeWindow
	}
	start, ok := calendar.ResolveForward(windowStart, time.Time{}, 0, location)
	if !ok {
		return ErrInvalidRequest
	}
	end, ok := calendar.ResolveForward(windowEnd, time.Time{}, 0, location)
	if !ok {
		return ErrInvalidRequest
	}
	allocations, err := repo.ListAllocations(ctx, userID, start, end)
	if err != nil {
		return err
	}
	rules, err := repo.ListRuleRevisionsForProjection(ctx, userID)
	if err != nil {
		return err
	}
	for _, rule := range rules {
		projected, err := projectedRuleOccurrences(ctx, repo, userID, rule, windowStart, throughDate)
		if err != nil {
			return err
		}
		for _, allocation := range projected {
			candidate := domain.Occurrence{RuleID: allocation.RuleID, ProjectID: allocation.ProjectID, OccurrenceDate: allocation.OccurrenceDate, StartAt: allocation.StartAt, EndAt: allocation.EndAt, Timezone: allocation.Timezone}
			if err := validateWithinActiveTime(candidate, activeWindowByDate); err != nil {
				return err
			}
		}
	}
	for _, allocation := range allocations {
		candidate := domain.Occurrence{RuleID: allocation.RuleID, ProjectID: allocation.ProjectID, OccurrenceDate: allocation.OccurrenceDate, StartAt: allocation.StartAt, EndAt: allocation.EndAt, Timezone: allocation.Timezone}
		if err := validateWithinActiveTime(candidate, activeWindowByDate); err != nil {
			return err
		}
	}
	return nil
}
