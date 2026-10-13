package usecase

import (
	"context"
	"errors"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/schedule/dao"
	"github.com/Najah7/task2todaytodo/internal/application/schedule/domain"
	"github.com/Najah7/task2todaytodo/internal/application/shared"
	"github.com/Najah7/task2todaytodo/internal/telemetry"
)

var (
	ErrTimezoneNotAllowed            = errors.New("timezone is not allowed")
	ErrTimezoneValidationUnavailable = errors.New("timezone validation is unavailable")
	ErrOccurrenceInvalid             = errors.New("initial occurrence cannot be resolved in timezone")
)

type CreateRuleUseCase struct {
	uow       UOW
	issueID   func() string
	timezones TimezoneValidator
	now       func() time.Time
	logger    telemetry.Logger
}

func NewCreateRuleUseCase(uow UOW, issueID func() string, timezones TimezoneValidator, logger telemetry.Logger, clocks ...func() time.Time) *CreateRuleUseCase {
	now := time.Now
	if len(clocks) > 0 && clocks[0] != nil {
		now = clocks[0]
	}
	return &CreateRuleUseCase{uow: uow, issueID: issueID, timezones: timezones, now: now, logger: telemetry.OrNop(logger)}
}

func (uc *CreateRuleUseCase) Execute(ctx context.Context, actorID string, input CreateRuleInput) (output dao.Rule, err error) {
	defer func() { logUnexpectedScheduleFailure(uc.logger, ctx, "CreateRuleUseCase.Execute", err) }()
	if actorID == "" {
		return dao.Rule{}, ErrPermissionDenied
	}
	if uc.timezones == nil {
		return dao.Rule{}, ErrTimezoneValidationUnavailable
	}
	allowed, err := uc.timezones.TimezoneExists(ctx, input.Timezone)
	if err != nil {
		return dao.Rule{}, err
	}
	if !allowed {
		return dao.Rule{}, ErrTimezoneNotAllowed
	}
	if _, err := time.LoadLocation(input.Timezone); err != nil {
		return dao.Rule{}, domain.ErrRecurrenceTimezoneInvalid
	}
	if input.AssigneeID == "" && input.ProjectID == "" {
		input.AssigneeID = actorID
	}
	weekdays, err := parseWeekdays(input.Weekdays)
	if err != nil {
		return dao.Rule{}, err
	}
	definition, err := domain.NewRuleDefinition(domain.RuleDefinition{
		Title: input.Title, Description: input.Description, Location: input.Location,
		ProjectID: input.ProjectID, AssigneeID: input.AssigneeID,
		LocalStart: input.LocalStart, LocalEnd: input.LocalEnd, Timezone: input.Timezone,
		AllDay: input.AllDay, Frequency: input.Frequency, Interval: input.Interval, Weekdays: weekdays, EndDate: input.EndDate,
	})
	if err != nil {
		return dao.Rule{}, err
	}
	ruleID, revisionID := uc.issueID(), uc.issueID()
	startDate := localCalendarDate(definition.LocalStart)
	anchorDate := startDate
	rule := domain.RecurrenceRule{
		Start: definition.LocalStart, AnchorDate: anchorDate, End: definition.LocalEnd,
		Timezone: definition.Timezone, Frequency: definition.Frequency, Interval: definition.Interval,
		Weekdays: weekdays, EndDate: definition.EndDate,
	}
	var firstOccurrence *dao.ScheduleOccurrence
	if definition.Frequency == domain.FrequencyOnce {
		occurrences, recurrenceErr := domain.GenerateOccurrences(rule, startDate, startDate)
		if recurrenceErr != nil {
			return dao.Rule{}, recurrenceErr
		}
		if len(occurrences) != 1 {
			return dao.Rule{}, ErrOccurrenceInvalid
		}
		id := uc.issueID()
		firstOccurrence = &dao.ScheduleOccurrence{
			ID: &id, RuleID: ruleID, OccurrenceDate: startDate.Format("2006-01-02"),
			SourceRevisionID: revisionID, AssigneeID: definition.AssigneeID, Title: definition.Title,
			Description: definition.Description, Location: definition.Location, ProjectID: definition.ProjectID,
			Timezone: definition.Timezone, AllDay: definition.AllDay, StartAt: occurrences[0].StartAt.Unix(), EndAt: occurrences[0].EndAt.Unix(),
		}
	}
	revision := dao.RuleRevision{
		Rule: dao.Rule{
			ID: ruleID, Revision: 1, EffectiveFrom: startDate.Format("2006-01-02"),
			RecurrenceAnchorDate: anchorDate.Format("2006-01-02"),
			LocalStart:           formatLocal(definition.LocalStart), LocalEnd: formatLocal(definition.LocalEnd),
			Timezone: definition.Timezone, AllDay: definition.AllDay, Frequency: definition.Frequency, Interval: definition.Interval,
			Weekdays: append([]string(nil), input.Weekdays...), EndDate: formatOptionalDate(definition.EndDate),
			Title: definition.Title, Description: definition.Description, Location: definition.Location,
			ProjectID: definition.ProjectID, AssigneeID: definition.AssigneeID,
		},
		RevisionID: revisionID, ChangedBy: actorID,
	}
	asOf := uc.now().UTC()
	err = doWithTodoImpact(ctx, uc.uow, TodoImpactScope{
		ActorID: actorID, RuleID: ruleID, OccurrenceDate: startDate.Format("2006-01-02"),
		Following: definition.Frequency != domain.FrequencyOnce, ProjectID: definition.ProjectID, AssigneeID: definition.AssigneeID,
	}, func(ctx context.Context, repos Repositories) error {
		var before *shared.ProjectWorkState
		if definition.ProjectID != "" {
			if err := repos.Schedules().LockProjectForScheduleMutation(ctx, domain.ProjectID(definition.ProjectID)); err != nil {
				return err
			}
			state, err := repos.ProjectLifecycle().CaptureWorkState(ctx, definition.ProjectID, asOf)
			if err != nil {
				return err
			}
			before = &state
		}
		created, err := repos.Rules().CreateRule(ctx, actorID, revision, firstOccurrence)
		if err != nil {
			return err
		}
		if before != nil {
			if err := repos.ProjectLifecycle().ReconcileWorkState(ctx, actorID, definition.ProjectID, *before, asOf); err != nil {
				return err
			}
			// Reconcile first: an occurrence eligible today may reopen a completed
			// Project. Only a Project that remains done should pause future work.
			status, err := repos.ProjectLifecycle().ReadStatus(ctx, definition.ProjectID)
			if err != nil {
				return err
			}
			if status == "done" && definition.Frequency != domain.FrequencyOnce {
				if err := repos.Rules().PauseProjectRules(ctx, actorID, definition.ProjectID, asOf); err != nil {
					return err
				}
			}
		}
		output = created
		return nil
	})
	if err != nil {
		return dao.Rule{}, err
	}
	logScheduleStateChange(uc.logger, ctx, "schedule.rule.create", domain.UserID(actorID), domain.ScheduleID(ruleID))
	return output, nil
}

func formatLocal(value time.Time) string {
	return value.Format("2006-01-02 15:04:05.999999999")
}

func formatOptionalDate(value *time.Time) *string {
	if value == nil {
		return nil
	}
	formatted := value.Format("2006-01-02")
	return &formatted
}
