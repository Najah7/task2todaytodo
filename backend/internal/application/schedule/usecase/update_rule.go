package usecase

import (
	"context"
	"errors"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/schedule/dao"
	"github.com/Najah7/task2todaytodo/internal/application/schedule/domain"
	"github.com/Najah7/task2todaytodo/internal/application/shared"
)

type UpdateRuleUseCase struct {
	uow       UOW
	issueID   func() string
	timezones TimezoneValidator
	now       func() time.Time
}

func NewUpdateRuleUseCase(uow UOW, issueID func() string, timezones TimezoneValidator, clocks ...func() time.Time) *UpdateRuleUseCase {
	now := time.Now
	if len(clocks) > 0 && clocks[0] != nil {
		now = clocks[0]
	}
	return &UpdateRuleUseCase{uow: uow, issueID: issueID, timezones: timezones, now: now}
}

func (uc *UpdateRuleUseCase) Execute(ctx context.Context, actorID, ruleID, occurrenceDate, scope string, input UpdateRuleInput, expectedRevision int32) (dao.Rule, error) {
	if scope != "following" {
		return dao.Rule{}, ErrScheduleScopeInvalid
	}
	selectedDate, err := time.Parse("2006-01-02", occurrenceDate)
	if err != nil {
		return dao.Rule{}, err
	}
	var result dao.Rule
	err = uc.uow.Do(ctx, func(ctx context.Context, repos Repositories) error {
		repo := repos.Rules()
		assignmentOnly := input.AssigneeID != nil && input.Title == nil && input.Description == nil && input.Location == nil &&
			input.ProjectID == nil && input.Timezone == nil && input.LocalStart == nil && input.LocalEnd == nil &&
			input.AllDay == nil && input.Frequency == nil && input.Interval == nil && input.Weekdays == nil && input.EndDate == nil
		writeCapability := shared.ScheduleUpdate()
		if assignmentOnly {
			writeCapability = shared.ScheduleAssignmentUpdate()
		}
		base, err := repo.GetEffectiveRevisionForDate(ctx, actorID, ruleID, occurrenceDate, writeCapability)
		if err != nil {
			return err
		}
		var currentOccurrence *dao.ScheduleOccurrence
		persisted, persistedErr := repo.GetPersistedOccurrenceForMutation(ctx, actorID, ruleID, occurrenceDate, writeCapability)
		if persistedErr == nil {
			currentOccurrence = &persisted
		} else if !errors.Is(persistedErr, ErrOccurrenceNotFound) {
			return persistedErr
		}
		latestRevision, err := repo.GetRuleRevisionForMutation(ctx, actorID, ruleID, occurrenceDate, writeCapability)
		if err != nil {
			return err
		}
		if latestRevision != expectedRevision {
			return ErrOccurrenceRevisionMismatch
		}
		definition := definitionFromRevision(base)
		originalStart := definition.LocalStart
		originalEnd := definition.LocalEnd
		dayOffset := int((localCalendarDate(originalEnd).Unix() - localCalendarDate(definition.LocalStart).Unix()) / 86400)
		originalRule := domain.RecurrenceRule{Start: originalStart, AnchorDate: parseDateValue(base.RecurrenceAnchorDate),
			End: originalEnd, Timezone: base.Timezone, Frequency: base.Frequency, Interval: base.Interval,
			Weekdays: mustParseWeekdays(base.Weekdays), EndDate: parseEndDate(base.EndDate)}
		originalOccurrences, err := domain.GenerateOccurrences(originalRule, selectedDate, selectedDate)
		if err != nil {
			return err
		}
		if len(originalOccurrences) == 0 || !originalOccurrences[0].StartAt.After(uc.now().UTC()) {
			return ErrOccurrenceInactive
		}
		applyRulePatch(&definition, input)
		if input.AllDay != nil && *input.AllDay && !base.AllDay {
			if definition.LocalEnd.Hour() != 0 || definition.LocalEnd.Minute() != 0 || definition.LocalEnd.Second() != 0 || definition.LocalEnd.Nanosecond() != 0 {
				dayOffset++
			}
			if dayOffset == 0 {
				dayOffset = 1
			}
		}
		startDate := selectedDate
		if input.LocalStart != nil && localCalendarDate(*input.LocalStart) != selectedDate {
			return ErrRescheduleDateMismatch
		}
		definition.LocalStart = wallTimeOnDate(definition.LocalStart, startDate)
		if input.LocalEnd == nil {
			endHour, endMinute, endSecond, endNanosecond := definition.LocalEnd.Hour(), definition.LocalEnd.Minute(), definition.LocalEnd.Second(), definition.LocalEnd.Nanosecond()
			if definition.AllDay {
				endHour, endMinute, endSecond, endNanosecond = 0, 0, 0, 0
			}
			definition.LocalEnd = time.Date(startDate.Year(), startDate.Month(), startDate.Day(), endHour, endMinute, endSecond, endNanosecond, time.UTC).AddDate(0, 0, dayOffset)
		} else if int((localCalendarDate(definition.LocalEnd).Unix()-localCalendarDate(definition.LocalStart).Unix())/86400) < 0 {
			return domain.ErrRuleTimeRange
		}
		if definition.AllDay {
			if input.AllDay != nil && *input.AllDay && input.LocalStart == nil {
				definition.LocalStart = time.Date(startDate.Year(), startDate.Month(), startDate.Day(), 0, 0, 0, 0, time.UTC)
			}
			if input.LocalEnd == nil {
				definition.LocalEnd = time.Date(startDate.Year(), startDate.Month(), startDate.Day(), 0, 0, 0, 0, time.UTC).AddDate(0, 0, dayOffset)
			}
		}
		if input.Frequency != nil && *input.Frequency != base.Frequency {
			definition.RecurrenceAnchorDate = selectedDate
		} else if base.RecurrenceAnchorDate != "" {
			definition.RecurrenceAnchorDate, _ = time.Parse("2006-01-02", base.RecurrenceAnchorDate)
		}
		if input.Frequency == nil {
			definition.Frequency = base.Frequency
		}
		if definition.Frequency == domain.FrequencyOnce {
			definition.Interval, definition.Weekdays, definition.EndDate = 0, nil, nil
		} else if definition.Interval == 0 {
			definition.Interval = 1
		}
		if definition.Frequency != domain.FrequencyWeekly {
			definition.Weekdays = nil
		}
		validated, err := domain.NewRuleDefinition(domain.RuleDefinition{
			Title: definition.Title, Description: definition.Description, Location: definition.Location,
			ProjectID: definition.ProjectID, AssigneeID: definition.AssigneeID,
			LocalStart: definition.LocalStart, LocalEnd: definition.LocalEnd, Timezone: definition.Timezone, AllDay: definition.AllDay,
			Frequency: definition.Frequency, Interval: definition.Interval, Weekdays: mustParseWeekdays(definition.Weekdays), EndDate: definition.EndDate,
		})
		if err != nil {
			return err
		}
		if uc.timezones == nil {
			return ErrTimezoneValidationUnavailable
		}
		allowed, err := uc.timezones.TimezoneExists(ctx, validated.Timezone)
		if err != nil {
			return err
		}
		if !allowed {
			return ErrTimezoneNotAllowed
		}
		if _, err := time.LoadLocation(validated.Timezone); err != nil {
			return domain.ErrRecurrenceTimezoneInvalid
		}
		projectChanged := validated.ProjectID != base.ProjectID
		if validated.AssigneeID != "" {
			eligible, err := repo.IsAssigneeEligible(ctx, validated.ProjectID, base.UserID, validated.AssigneeID)
			if err != nil {
				return err
			}
			if !eligible && projectChanged && input.AssigneeID == nil {
				validated.AssigneeID = base.UserID
				eligible, err = repo.IsAssigneeEligible(ctx, validated.ProjectID, base.UserID, validated.AssigneeID)
				if err != nil {
					return err
				}
			}
			if !eligible {
				return ErrScheduleAssigneeNotEligible
			}
		}
		assigneeChanged := validated.AssigneeID != base.AssigneeID
		if assigneeChanged && !assignmentOnly {
			if _, err := repo.GetEffectiveRevisionForDate(ctx, actorID, ruleID, occurrenceDate, shared.ScheduleAssignmentUpdate()); err != nil {
				return ErrPermissionDenied
			}
		}
		if projectChanged && validated.ProjectID != "" {
			allowed, err := repos.Schedules().CheckProjectPermission(ctx, domain.UserID(actorID), domain.ProjectID(validated.ProjectID), shared.ScheduleCreate())
			if err != nil {
				return err
			}
			if !allowed {
				return ErrPermissionDenied
			}
		}
		anchor := selectedDate
		if input.Frequency == nil || *input.Frequency == base.Frequency {
			if parsed, parseErr := time.Parse("2006-01-02", base.RecurrenceAnchorDate); parseErr == nil {
				anchor = parsed
			}
		}
		weekdays := append([]string{}, definition.Weekdays...)
		var cutoffAfter *string
		if input.EndDate == nil {
			cutoffAfter = base.CutoffAfter
		}
		localStart := formatLocal(validated.LocalStart)
		localEnd := formatLocal(validated.LocalEnd)
		newRevision := dao.RuleRevision{Rule: dao.Rule{
			ID: ruleID, UserID: base.UserID, Revision: latestRevision + 1,
			EffectiveFrom: occurrenceDate, RecurrenceAnchorDate: anchor.Format("2006-01-02"),
			LocalStart: localStart, LocalEnd: localEnd, Timezone: validated.Timezone, AllDay: validated.AllDay,
			Frequency: validated.Frequency, Interval: validated.Interval, Weekdays: weekdays,
			EndDate: formatOptionalDate(validated.EndDate), CutoffAfter: cutoffAfter, Title: validated.Title,
			Description: validated.Description, Location: validated.Location, ProjectID: validated.ProjectID, AssigneeID: validated.AssigneeID,
		}, RevisionID: uc.issueID(), ChangedBy: actorID}
		projectIDs := uniqueProjectIDs([]dao.RuleRevision{
			{Rule: dao.Rule{ProjectID: base.ProjectID}},
			{Rule: dao.Rule{ProjectID: validated.ProjectID}},
		})
		beforeStates := make(map[string]shared.ProjectWorkState, len(projectIDs))
		for _, projectID := range projectIDs {
			if err := repos.Schedules().LockProjectForScheduleMutation(ctx, domain.ProjectID(projectID)); err != nil {
				return err
			}
			state, err := repos.ProjectLifecycle().CaptureWorkState(ctx, projectID, uc.now().UTC())
			if err != nil {
				return err
			}
			beforeStates[projectID] = state
		}
		if err := repo.LockRuleForRevisionMutation(ctx, actorID, ruleID, occurrenceDate, writeCapability); err != nil {
			return err
		}
		fresh, err := repo.GetEffectiveRevisionForDate(ctx, actorID, ruleID, occurrenceDate, writeCapability)
		if err != nil {
			return err
		}
		freshRevision, err := repo.GetRuleRevisionForMutation(ctx, actorID, ruleID, occurrenceDate, writeCapability)
		if err != nil {
			return err
		}
		if fresh.RevisionID != base.RevisionID || freshRevision != expectedRevision {
			return ErrOccurrenceRevisionMismatch
		}
		lockedOccurrence, occurrenceErr := repo.GetPersistedOccurrenceForMutation(ctx, actorID, ruleID, occurrenceDate, writeCapability)
		if occurrenceErr == nil {
			if currentOccurrence != nil && lockedOccurrence.Revision != currentOccurrence.Revision {
				return ErrOccurrenceRevisionMismatch
			}
			currentOccurrence = &lockedOccurrence
		} else if errors.Is(occurrenceErr, ErrOccurrenceNotFound) {
			currentOccurrence = nil
		} else {
			return occurrenceErr
		}
		if projectChanged && validated.ProjectID != "" {
			allowed, err := repos.Schedules().CheckProjectPermission(ctx, domain.UserID(actorID), domain.ProjectID(validated.ProjectID), shared.ScheduleCreate())
			if err != nil {
				return err
			}
			if !allowed {
				return ErrPermissionDenied
			}
		}
		if validated.AssigneeID != "" {
			eligible, err := repo.IsAssigneeEligible(ctx, validated.ProjectID, base.UserID, validated.AssigneeID)
			if err != nil {
				return err
			}
			if !eligible {
				return ErrScheduleAssigneeNotEligible
			}
		}
		result, err = repo.PublishRevision(ctx, actorID, ruleID, newRevision, expectedRevision, writeCapability)
		if err != nil {
			return err
		}
		result.CreatedAt = base.CreatedAt
		if currentOccurrence != nil && currentOccurrence.ProjectID == base.ProjectID {
			occurrences, err := domain.GenerateOccurrences(domain.RecurrenceRule{
				Start: validated.LocalStart, AnchorDate: anchor, End: validated.LocalEnd, Timezone: validated.Timezone,
				Frequency: validated.Frequency, Interval: validated.Interval, Weekdays: mustParseWeekdays(weekdays), EndDate: validated.EndDate,
			}, selectedDate, selectedDate)
			if err != nil {
				return err
			}
			if len(occurrences) != 1 {
				return ErrOccurrenceInvalid
			}
			row := dao.ScheduleOccurrence{ID: currentOccurrence.ID, RuleID: ruleID, OccurrenceDate: occurrenceDate,
				SourceRevisionID: newRevision.RevisionID, UserID: base.UserID, ProjectID: validated.ProjectID,
				AssigneeID: validated.AssigneeID, Title: validated.Title, Description: validated.Description,
				Location: validated.Location, Timezone: validated.Timezone, AllDay: validated.AllDay, StartAt: occurrences[0].StartAt.Unix(),
				EndAt: occurrences[0].EndAt.Unix(), Revision: currentOccurrence.Revision}
			if err := repo.ApplyRuleRevisionToOccurrence(ctx, actorID, row); err != nil {
				return err
			}
		} else if validated.Frequency == domain.FrequencyOnce && currentOccurrence == nil {
			occurrences, err := domain.GenerateOccurrences(domain.RecurrenceRule{
				Start: validated.LocalStart, AnchorDate: anchor, End: validated.LocalEnd, Timezone: validated.Timezone,
				Frequency: validated.Frequency, Interval: 0,
			}, selectedDate, selectedDate)
			if err != nil {
				return err
			}
			if len(occurrences) != 1 {
				return ErrOccurrenceInvalid
			}
			from, to := selectedDate.AddDate(0, 0, -2).UTC(), selectedDate.AddDate(0, 0, 3).UTC()
			origins, err := repo.ListOccurrenceOrigins(ctx, actorID, []string{ruleID}, from, to)
			if err != nil {
				return err
			}
			for _, origin := range origins {
				if origin.OccurrenceDate == occurrenceDate {
					return nil
				}
			}
			id := uc.issueID()
			row := dao.ScheduleOccurrence{ID: &id, RuleID: ruleID, OccurrenceDate: occurrenceDate,
				SourceRevisionID: newRevision.RevisionID, UserID: base.UserID, ProjectID: validated.ProjectID,
				AssigneeID: validated.AssigneeID, Title: validated.Title, Description: validated.Description,
				Location: validated.Location, Timezone: validated.Timezone, AllDay: validated.AllDay,
				StartAt: occurrences[0].StartAt.Unix(), EndAt: occurrences[0].EndAt.Unix()}
			if _, err := repo.MaterializeOccurrence(ctx, actorID, row, writeCapability); err != nil {
				return err
			}
		}
		for _, projectID := range projectIDs {
			if err := repos.ProjectLifecycle().ReconcileWorkState(ctx, actorID, projectID, beforeStates[projectID], uc.now().UTC()); err != nil {
				return err
			}
		}
		if validated.ProjectID != "" && validated.Frequency != domain.FrequencyOnce {
			status, err := repos.ProjectLifecycle().ReadStatus(ctx, validated.ProjectID)
			if err != nil {
				return err
			}
			if status == "done" {
				if err := repo.PauseProjectRules(ctx, actorID, validated.ProjectID, uc.now().UTC()); err != nil {
					return err
				}
			}
		}
		return nil
	})
	return result, err
}

type ruleDefinition struct {
	UserID, Title, Description, Location, ProjectID, AssigneeID string
	Timezone, Frequency                                         string
	Interval                                                    int
	Weekdays                                                    []string
	EndDate                                                     *time.Time
	LocalStart, LocalEnd, RecurrenceAnchorDate                  time.Time
	AllDay                                                      bool
}

func definitionFromRevision(row dao.RuleRevision) ruleDefinition {
	start := parseLocalTimestamp(row.LocalStart)
	end := parseLocalTimestamp(row.LocalEnd)
	anchor, _ := time.Parse("2006-01-02", row.RecurrenceAnchorDate)
	return ruleDefinition{UserID: row.UserID, Title: row.Title, Description: row.Description, Location: row.Location,
		ProjectID: row.ProjectID, AssigneeID: row.AssigneeID, Timezone: row.Timezone, Frequency: row.Frequency,
		Interval: row.Interval, Weekdays: append([]string{}, row.Weekdays...), EndDate: parseEndDate(row.EndDate),
		LocalStart: start, LocalEnd: end, RecurrenceAnchorDate: anchor, AllDay: row.AllDay}
}

func applyRulePatch(definition *ruleDefinition, patch UpdateRuleInput) {
	if patch.Title != nil {
		definition.Title = *patch.Title
	}
	if patch.Description != nil {
		definition.Description = *patch.Description
	}
	if patch.Location != nil {
		definition.Location = *patch.Location
	}
	if patch.ProjectID != nil {
		definition.ProjectID = *patch.ProjectID
	}
	if patch.AssigneeID != nil {
		definition.AssigneeID = *patch.AssigneeID
	}
	if patch.Timezone != nil {
		definition.Timezone = *patch.Timezone
	}
	if patch.LocalStart != nil {
		definition.LocalStart = *patch.LocalStart
	}
	if patch.LocalEnd != nil {
		definition.LocalEnd = *patch.LocalEnd
	}
	if patch.AllDay != nil {
		definition.AllDay = *patch.AllDay
	}
	if patch.Frequency != nil {
		definition.Frequency = *patch.Frequency
	}
	if patch.Interval != nil {
		definition.Interval = *patch.Interval
	}
	if patch.Weekdays != nil {
		definition.Weekdays = append([]string{}, (*patch.Weekdays)...)
	}
	if patch.EndDate != nil {
		definition.EndDate = *patch.EndDate
	}
}

func wallTimeOnDate(value, date time.Time) time.Time {
	return time.Date(date.Year(), date.Month(), date.Day(), value.Hour(), value.Minute(), value.Second(), value.Nanosecond(), time.UTC)
}

func parseDateValue(value string) time.Time {
	date, _ := time.Parse("2006-01-02", value)
	return date
}

func parseEndDate(value *string) *time.Time {
	if value == nil {
		return nil
	}
	date, err := time.Parse("2006-01-02", *value)
	if err != nil {
		return nil
	}
	return &date
}

func mustParseWeekdays(values []string) []time.Weekday {
	out, err := parseWeekdays(values)
	if err != nil {
		return nil
	}
	return out
}
