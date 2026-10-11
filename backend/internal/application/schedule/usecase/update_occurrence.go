package usecase

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/schedule/dao"
	"github.com/Najah7/task2todaytodo/internal/application/schedule/domain"
	"github.com/Najah7/task2todaytodo/internal/application/shared"
	"github.com/Najah7/task2todaytodo/internal/telemetry"
)

var (
	ErrOccurrenceRevisionMismatch       = errors.New("schedule occurrence revision does not match")
	ErrOccurrenceTimeInvalid            = errors.New("occurrence end must be after start")
	ErrOccurrenceTitleEmpty             = errors.New("occurrence title cannot be empty")
	ErrOccurrenceIDGeneratorUnavailable = errors.New("occurrence ID generator unavailable")
	ErrBlueprintIDGeneratorUnavailable  = errors.New("blueprint revision ID generator unavailable")
)

type UpdateOccurrenceUseCase struct {
	uow     UOW
	logger  telemetry.Logger
	issueID func() string
	now     func() time.Time
}

func NewUpdateOccurrenceUseCase(uow UOW, logger telemetry.Logger, issueID func() string) *UpdateOccurrenceUseCase {
	return &UpdateOccurrenceUseCase{uow: uow, logger: telemetry.OrNop(logger), issueID: issueID, now: time.Now}
}

func (uc *UpdateOccurrenceUseCase) Execute(ctx context.Context, actorID, blueprintID, occurrenceDate string, input UpdateScheduleInput, expectedRevision int32) (dao.ScheduleOccurrence, error) {
	return uc.execute(ctx, actorID, blueprintID, occurrenceDate, input, expectedRevision, "schedule.occurrence.update")
}

func (uc *UpdateOccurrenceUseCase) execute(ctx context.Context, actorID, blueprintID, occurrenceDate string, input UpdateScheduleInput, expectedRevision int32, operation string) (output dao.ScheduleOccurrence, err error) {
	defer func() { logUnexpectedScheduleFailure(uc.logger, ctx, "UpdateOccurrenceUseCase.Execute", err) }()
	err = uc.uow.Do(ctx, func(ctx context.Context, repos Repositories) error {
		capability := occurrenceUpdateCapability(input)
		initial, err := resolveOccurrenceForMutation(ctx, repos.Blueprints(), actorID, blueprintID, occurrenceDate, capability, false)
		if err != nil {
			return err
		}
		if initial.value.Revision != expectedRevision {
			return ErrOccurrenceRevisionMismatch
		}
		if err := validateOccurrencePatch(input); err != nil {
			return err
		}
		if err := authorizeOccurrencePatch(ctx, repos, actorID, initial.value, input); err != nil {
			return err
		}
		targetProject := initial.value.ProjectID
		if input.ProjectID != nil {
			targetProject = *input.ProjectID
		}
		current, before, err := lockAndReloadOccurrence(ctx, repos, actorID, blueprintID, occurrenceDate, initial, expectedRevision, capability,
			[]string{initial.value.ProjectID, targetProject}, uc.now().UTC(), false)
		if err != nil {
			return err
		}
		if err := authorizeOccurrencePatch(ctx, repos, actorID, current.value, input); err != nil {
			return err
		}
		updated := current.value
		applyOccurrencePatch(&updated, input)
		updated.AssigneeID, err = occurrenceTargetAssignee(ctx, repos, current.value, input)
		if err != nil {
			return err
		}
		if updated.EndAt <= updated.StartAt {
			return ErrOccurrenceTimeInvalid
		}
		if strings.TrimSpace(updated.Title) == "" {
			return ErrOccurrenceTitleEmpty
		}
		if !occurrencePatchChanges(current.value, updated) {
			output = current.value
			return nil
		}
		updated.IsOverride = true
		if !current.persisted {
			id, err := virtualOccurrenceID(uc.issueID)
			if err != nil {
				return err
			}
			updated.ID = &id
			output, err = repos.Blueprints().MaterializeOccurrence(ctx, actorID, updated, capability)
		} else {
			output, err = repos.Blueprints().UpdatePersistedOccurrence(ctx, actorID, updated, expectedRevision, capability)
		}
		if err != nil {
			return mapOccurrenceConflict(err)
		}
		return finishScheduleProjectMutation(ctx, repos, domain.UserID(actorID), before, uc.now().UTC())
	})
	if err == nil {
		logScheduleStateChange(uc.logger, ctx, operation, domain.UserID(actorID), domain.ScheduleID(blueprintID))
	}
	return output, err
}

func occurrenceUpdateCapability(input UpdateScheduleInput) shared.Capability {
	contentChange := input.Title != nil || input.Description != nil || input.Location != nil || input.ProjectID != nil || input.StartAt != nil || input.EndAt != nil
	if contentChange || input.AssigneeID == nil {
		return shared.ScheduleUpdate()
	}
	return shared.ScheduleAssignmentUpdate()
}

func validateOccurrencePatch(input UpdateScheduleInput) error {
	if input.Title != nil && strings.TrimSpace(*input.Title) == "" {
		return ErrOccurrenceTitleEmpty
	}
	if input.StartAt != nil && input.EndAt != nil && !input.EndAt.After(*input.StartAt) {
		return ErrOccurrenceTimeInvalid
	}
	return nil
}

func authorizeOccurrencePatch(ctx context.Context, repos Repositories, actorID string, current dao.ScheduleOccurrence, input UpdateScheduleInput) error {
	projectID := current.ProjectID
	if input.ProjectID != nil {
		projectID = *input.ProjectID
	}
	if projectID != current.ProjectID {
		if current.ProjectID != "" {
			if err := requireScheduleProjectPermission(ctx, repos.Schedules(), domain.UserID(actorID), domain.ProjectID(current.ProjectID), shared.ScheduleUpdate()); err != nil {
				return err
			}
		}
		if projectID != "" {
			if err := requireScheduleProjectPermission(ctx, repos.Schedules(), domain.UserID(actorID), domain.ProjectID(projectID), shared.ScheduleCreate()); err != nil {
				return err
			}
		}
	}
	assigneeID, err := occurrenceTargetAssignee(ctx, repos, current, input)
	if err != nil {
		return err
	}
	if assigneeID == "" {
		return ErrScheduleAssigneeNotEligible
	}
	if assigneeID != current.AssigneeID {
		if projectID != "" {
			if err := requireScheduleProjectPermission(ctx, repos.Schedules(), domain.UserID(actorID), domain.ProjectID(projectID), shared.ScheduleAssignmentUpdate()); err != nil {
				return err
			}
		} else if assigneeID != current.UserID {
			return ErrScheduleAssigneeNotEligible
		}
	}
	if projectID == "" {
		if assigneeID != current.UserID {
			return ErrScheduleAssigneeNotEligible
		}
		return nil
	}
	allowed, err := repos.Blueprints().IsAssigneeEligible(ctx, projectID, current.UserID, assigneeID)
	if err != nil {
		return err
	}
	if !allowed {
		return ErrScheduleAssigneeNotEligible
	}
	return nil
}

func occurrenceTargetAssignee(ctx context.Context, repos Repositories, current dao.ScheduleOccurrence, input UpdateScheduleInput) (string, error) {
	projectID := current.ProjectID
	if input.ProjectID != nil {
		projectID = *input.ProjectID
	}
	if input.AssigneeID != nil {
		return *input.AssigneeID, nil
	}
	assigneeID := current.AssigneeID
	if projectID == current.ProjectID {
		return assigneeID, nil
	}
	if projectID == "" {
		return current.UserID, nil
	}
	allowed, err := repos.Blueprints().IsAssigneeEligible(ctx, projectID, current.UserID, assigneeID)
	if err != nil {
		return "", err
	}
	if allowed {
		return assigneeID, nil
	}
	ownerAllowed, err := repos.Blueprints().IsAssigneeEligible(ctx, projectID, current.UserID, current.UserID)
	if err != nil {
		return "", err
	}
	if !ownerAllowed {
		return "", ErrScheduleAssigneeNotEligible
	}
	return current.UserID, nil
}

func occurrencePatchChanges(before, after dao.ScheduleOccurrence) bool {
	return before.Title != after.Title || before.Description != after.Description || before.Location != after.Location ||
		before.ProjectID != after.ProjectID || before.AssigneeID != after.AssigneeID ||
		before.StartAt != after.StartAt || before.EndAt != after.EndAt
}

func applyOccurrencePatch(current *dao.ScheduleOccurrence, input UpdateScheduleInput) {
	if input.Title != nil {
		current.Title = *input.Title
	}
	if input.Description != nil {
		current.Description = *input.Description
	}
	if input.Location != nil {
		current.Location = *input.Location
	}
	if input.ProjectID != nil {
		current.ProjectID = *input.ProjectID
	}
	if input.AssigneeID != nil {
		current.AssigneeID = *input.AssigneeID
	}
	if input.StartAt != nil {
		current.StartAt = input.StartAt.UTC().Unix()
	}
	if input.EndAt != nil {
		current.EndAt = input.EndAt.UTC().Unix()
	}
}

func mapOccurrenceConflict(err error) error {
	if errors.Is(err, ErrOccurrenceRevisionMismatch) {
		return ErrOccurrenceRevisionMismatch
	}
	return err
}
