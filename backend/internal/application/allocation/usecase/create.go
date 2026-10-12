package usecase

import (
	"context"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/allocation/dao"
	"github.com/Najah7/task2todaytodo/internal/application/allocation/domain"
	"github.com/Najah7/task2todaytodo/internal/application/shared/calendar"
)

type CreateAllocationUseCase struct {
	repo    Repository
	uow     UnitOfWork
	issueID func() string
	now     func() time.Time
}

func NewCreateAllocationUseCase(repo Repository, uow UnitOfWork, issueID func() string, now func() time.Time) *CreateAllocationUseCase {
	return &CreateAllocationUseCase{repo: repo, uow: uow, issueID: issueID, now: now}
}

func (uc *CreateAllocationUseCase) Execute(ctx context.Context, userID, projectID, timezone string, startAt, endAt time.Time, repeatWeekly bool) (dao.Occurrence, error) {
	if userID == "" || projectID == "" || !endAt.After(startAt) || uc.issueID == nil {
		return dao.Occurrence{}, ErrInvalidRequest
	}
	profile, err := uc.repo.GetProfile(ctx, userID)
	if err != nil {
		return dao.Occurrence{}, err
	}
	if timezone == "" {
		timezone = profile.Timezone
	}
	location, err := time.LoadLocation(timezone)
	if err != nil {
		return dao.Occurrence{}, ErrInvalidRequest
	}
	localStart, localEnd := startAt.In(location), endAt.In(location)
	startDate := calendar.DateOnly(localStart)
	endDate := calendar.DateOnly(localEnd)
	endOffset := int(calendar.DateOnly(endDate).Sub(calendar.DateOnly(startDate)).Hours() / 24)
	if endOffset < 0 || endOffset > 1 {
		return dao.Occurrence{}, ErrInvalidRequest
	}
	frequency := "once"
	if repeatWeekly {
		frequency = "weekly"
	}
	slot, err := domain.NewSlot(int(localStart.Weekday()), localStart.Format("15:04"), localEnd.Format("15:04"), endOffset, timezone, frequency)
	if err != nil {
		return dao.Occurrence{}, ErrInvalidRequest
	}
	first, err := slot.At(startDate)
	if err != nil {
		return dao.Occurrence{}, ErrInvalidRequest
	}
	if !first.Start.Equal(startAt) || !first.End.Equal(endAt) {
		return dao.Occurrence{}, ErrInvalidRequest
	}
	now := uc.now().UTC()
	ruleID, sourceID, occurrenceID := uc.issueID(), uc.issueID(), uc.issueID()
	dateThrough := startDate.AddDate(0, 0, 400)
	candidates := []domain.Occurrence{{ID: occurrenceID, RuleID: ruleID, SourceRevisionID: sourceID, ProjectID: projectID, OccurrenceDate: startDate, StartAt: startAt.UTC(), EndAt: endAt.UTC(), Timezone: timezone}}
	if repeatWeekly {
		recurrence := recurrenceFor(ruleID, sourceID, projectID, slot, startDate)
		generated, err := calendar.GenerateOccurrences(recurrence, startDate, dateThrough)
		if err != nil {
			return dao.Occurrence{}, ErrInvalidRequest
		}
		candidates = make([]domain.Occurrence, 0, len(generated))
		for i, item := range generated {
			id := occurrenceID
			if i > 0 {
				id = uc.issueID()
			}
			candidates = append(candidates, domain.Occurrence{ID: id, RuleID: ruleID, SourceRevisionID: sourceID, ProjectID: projectID, OccurrenceDate: item.Date, StartAt: item.StartAt.UTC(), EndAt: item.EndAt.UTC(), Timezone: timezone})
		}
	}
	var output dao.Occurrence
	err = uc.uow.Do(ctx, func(ctx context.Context, repo Repository) error {
		if err := repo.LockProjectsForMutation(ctx, userID, []string{projectID}); err != nil {
			return err
		}
		if err := repo.LockUser(ctx, userID); err != nil {
			return err
		}
		projectStatus, err := repo.GetProjectStatus(ctx, userID, projectID)
		if err != nil {
			return err
		}
		if projectStatus != "done" {
			if err := validateAvailability(ctx, repo, userID, candidates, "", now); err != nil {
				return err
			}
		}
		if err := repo.CreateRule(ctx, userID, ruleID, projectID); err != nil {
			return err
		}
		if err := repo.CreateRuleRevision(ctx, userID, ruleID, projectID, sourceID, 1, startDate, startDate, now, slot); err != nil {
			return err
		}
		if projectStatus == "done" {
			if err := repo.SetRulePause(ctx, userID, ruleID, "project_completed", now); err != nil {
				return err
			}
			if err := repo.StartPauseInterval(ctx, projectID, ruleID, now); err != nil {
				return err
			}
		}
		stored, err := repo.InsertOccurrence(ctx, userID, candidates[0])
		if err != nil {
			return err
		}
		output = occurrenceFromAllocation(stored)
		return nil
	})
	return output, err
}

func recurrenceFor(ruleID, sourceID, projectID string, slot domain.Slot, startDate time.Time) calendar.RecurrenceRule {
	start := time.Date(startDate.Year(), startDate.Month(), startDate.Day(), slot.Start.Hour(), slot.Start.Minute(), slot.Start.Second(), slot.Start.Nanosecond(), time.UTC)
	endDate := startDate.AddDate(0, 0, slot.EndDayOffset)
	end := time.Date(endDate.Year(), endDate.Month(), endDate.Day(), slot.End.Hour(), slot.End.Minute(), slot.End.Second(), slot.End.Nanosecond(), time.UTC)
	weekdays := []time.Weekday{time.Weekday(slot.Weekday)}
	return calendar.RecurrenceRule{Start: start, AnchorDate: startDate, End: end, Timezone: slot.Timezone, Frequency: slot.Frequency, Interval: 1, Weekdays: weekdays}
}

func occurrenceFromAllocation(value dao.Allocation) dao.Occurrence {
	return dao.Occurrence{ID: value.ID, RuleID: value.RuleID, OccurrenceDate: value.OccurrenceDate, SourceRevisionID: value.SourceRevisionID, ProjectID: value.ProjectID, ProjectTitle: value.ProjectTitle, Timezone: value.Timezone, StartAt: value.StartAt, EndAt: value.EndAt, Revision: value.Revision, RuleRevision: value.RuleRevision, IsException: value.IsException, CanUpdate: value.CanUpdate, CanDelete: value.CanDelete, CanUpdateFollowing: value.CanUpdateFollowing, CanDeleteFollowing: value.CanDeleteFollowing}
}
