package usecase

import (
	"context"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/allocation/dao"
	"github.com/Najah7/task2todaytodo/internal/application/allocation/domain"
	"github.com/Najah7/task2todaytodo/internal/application/shared/calendar"
)

type PutSleepExceptionUseCase struct {
	repo Repository
	uow  UnitOfWork
	now  func() time.Time
}

func NewPutSleepExceptionUseCase(repo Repository, uow UnitOfWork, now func() time.Time) *PutSleepExceptionUseCase {
	return &PutSleepExceptionUseCase{repo: repo, uow: uow, now: now}
}

func (uc *PutSleepExceptionUseCase) Execute(ctx context.Context, userID string, date time.Time, expected int32, wakeLocal, bedtimeLocal time.Time, timezone string) (dao.SleepException, error) {
	if userID == "" || expected < 0 || timezone == "" {
		return dao.SleepException{}, ErrInvalidRequest
	}
	date = calendar.DateOnly(date)
	if calendar.DateOnly(wakeLocal).Format("2006-01-02") != date.Format("2006-01-02") || calendar.DateOnly(bedtimeLocal).Before(date) || calendar.DateOnly(bedtimeLocal).After(date.AddDate(0, 0, 1)) {
		return dao.SleepException{}, ErrInvalidRequest
	}
	exception := domain.SleepException{LocalDate: date, WakeLocal: wakeLocal, BedtimeLocal: bedtimeLocal, Timezone: timezone}
	var result dao.SleepException
	err := uc.uow.Do(ctx, func(ctx context.Context, repo Repository) error {
		if err := repo.LockUser(ctx, userID); err != nil {
			return err
		}
		if err := validateSleepChange(ctx, repo, userID, exception, date, uc.now()); err != nil {
			return err
		}
		stored, err := repo.SaveSleepException(ctx, userID, expected, exception)
		if err != nil {
			return err
		}
		result = stored
		return nil
	})
	return result, err
}

type DeleteSleepExceptionUseCase struct {
	repo Repository
	uow  UnitOfWork
	now  func() time.Time
}

func NewDeleteSleepExceptionUseCase(repo Repository, uow UnitOfWork, now func() time.Time) *DeleteSleepExceptionUseCase {
	if now == nil {
		now = time.Now
	}
	return &DeleteSleepExceptionUseCase{repo: repo, uow: uow, now: now}
}

func (uc *DeleteSleepExceptionUseCase) Execute(ctx context.Context, userID string, date time.Time, expected int32) error {
	if userID == "" || expected <= 0 {
		return ErrInvalidRequest
	}
	date = calendar.DateOnly(date)
	return uc.uow.Do(ctx, func(ctx context.Context, repo Repository) error {
		if err := repo.LockUser(ctx, userID); err != nil {
			return err
		}
		reset := domain.SleepException{LocalDate: date}
		if err := validateSleepChange(ctx, repo, userID, reset, date, uc.now()); err != nil {
			return err
		}
		return repo.DeleteSleepException(ctx, userID, date, expected)
	})
}

func validateSleepChange(ctx context.Context, repo Repository, userID string, replacement domain.SleepException, changedDate time.Time, now time.Time) error {
	profile, err := repo.GetProfile(ctx, userID)
	if err != nil {
		return err
	}
	location, err := time.LoadLocation(profile.Timezone)
	if err != nil {
		return ErrInvalidRequest
	}
	fromDate, throughDate := changedDate.AddDate(0, 0, -3), changedDate.AddDate(0, 0, 400)
	exceptions, err := repo.ListSleepExceptions(ctx, userID, fromDate, throughDate)
	if err != nil {
		return err
	}
	byDate := make(map[string]dao.SleepException, len(exceptions)+1)
	for _, item := range exceptions {
		byDate[item.LocalDate.Format("2006-01-02")] = item
	}
	if replacement.Timezone == "" {
		delete(byDate, changedDate.Format("2006-01-02"))
	} else {
		byDate[changedDate.Format("2006-01-02")] = dao.SleepException{LocalDate: changedDate, WakeLocal: replacement.WakeLocal, BedtimeLocal: replacement.BedtimeLocal, Timezone: replacement.Timezone, HasException: true}
	}
	wakeDefault, _ := time.Parse("15:04", profile.WakeUpTime)
	bedDefault, _ := time.Parse("15:04", profile.Bedtime)
	var prior domain.Interval
	first := true
	sleep := make([]domain.Interval, 0, 405)
	for date := fromDate; !date.After(throughDate); date = date.AddDate(0, 0, 1) {
		wake, bed, zone := wakeDefault, bedDefault, profile.Timezone
		var awake domain.Interval
		if item, ok := byDate[date.Format("2006-01-02")]; ok {
			awake, err = domain.AwakeIntervalFromLocalPair(date, item.WakeLocal, item.BedtimeLocal, item.Timezone)
		} else {
			awake, err = domain.AwakeInterval(date, wake, bed, zone)
		}
		if err != nil {
			return ErrInvalidRequest
		}
		if !first && awake.Start.Before(prior.End) {
			return ErrInvalidRequest
		}
		if !first && awake.Start.After(prior.End) {
			sleep = append(sleep, domain.Interval{Start: prior.End, End: awake.Start})
		}
		first = false
		prior = awake
	}
	if !throughDate.After(changedDate) {
		return nil
	}
	startDate := calendar.DateOnly(now.In(location))
	start, ok := calendar.ResolveForward(startDate, time.Time{}, 0, location)
	if !ok {
		return ErrInvalidRequest
	}
	end, ok := calendar.ResolveForward(throughDate.AddDate(0, 0, 1), time.Time{}, 0, location)
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
		projected, err := projectedRuleOccurrences(ctx, repo, userID, rule, startDate.AddDate(0, 0, -1), throughDate.AddDate(0, 0, 1))
		if err != nil {
			return err
		}
		for _, allocation := range projected {
			if allocation.StartAt.Before(now) {
				continue
			}
			candidate := domain.Occurrence{RuleID: allocation.RuleID, ProjectID: allocation.ProjectID, OccurrenceDate: allocation.OccurrenceDate, StartAt: allocation.StartAt, EndAt: allocation.EndAt, Timezone: allocation.Timezone}
			for _, band := range sleep {
				if calendar.Overlaps(candidate.StartAt, candidate.EndAt, band.Start, band.End) {
					return conflict("sleep_conflict", candidate)
				}
			}
		}
	}
	for _, allocation := range allocations {
		if allocation.EndAt.Before(now) {
			continue
		}
		for _, band := range sleep {
			if calendar.Overlaps(allocation.StartAt, allocation.EndAt, band.Start, band.End) {
				return conflictFromRow("sleep_conflict", allocation)
			}
		}
	}
	return nil
}
