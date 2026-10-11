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

// SetOccurrenceStateUseCase applies one named state transition to an occurrence.
// State is fixed at construction so each public action retains its own ACL.
type SetOccurrenceStateUseCase struct {
	uow     UOW
	state   string
	logger  telemetry.Logger
	issueID func() string
	now     func() time.Time
}

func NewSetOccurrenceStateUseCase(uow UOW, state string, logger telemetry.Logger, issueID func() string) *SetOccurrenceStateUseCase {
	return &SetOccurrenceStateUseCase{uow: uow, state: state, logger: telemetry.OrNop(logger), issueID: issueID, now: time.Now}
}

func (uc *SetOccurrenceStateUseCase) Execute(ctx context.Context, actorID, blueprintID, occurrenceDate string, expectedRevision int32) (err error) {
	defer func() { logUnexpectedScheduleFailure(uc.logger, ctx, "SetOccurrenceStateUseCase.Execute", err) }()
	capability, err := occurrenceStateCapability(uc.state)
	if err != nil {
		return err
	}
	err = uc.uow.Do(ctx, func(ctx context.Context, repos Repositories) error {
		includeInactive := uc.state == "restore" || uc.state == "skip"
		initial, err := resolveOccurrenceForMutation(ctx, repos.Blueprints(), actorID, blueprintID, occurrenceDate, capability, includeInactive)
		if err != nil {
			return err
		}
		if initial.value.Revision != expectedRevision {
			return ErrOccurrenceRevisionMismatch
		}
		if uc.state == "skip" && initial.value.Skipped {
			return nil
		}
		if uc.state == "restore" && initial.value.Deleted {
			return ErrOccurrenceInactive
		}
		if uc.state == "restore" && !initial.persisted {
			return ErrOccurrenceNotFound
		}
		current, before, err := lockAndReloadOccurrence(ctx, repos, actorID, blueprintID, occurrenceDate, initial, expectedRevision, capability,
			[]string{initial.value.ProjectID}, uc.now().UTC(), includeInactive)
		if err != nil {
			return err
		}
		if err := validateOccurrenceState(current.value, uc.state); err != nil {
			return err
		}
		updated := current.value
		switch uc.state {
		case "complete":
			updated.Completed = true
		case "reopen":
			updated.Completed = false
		case "skip":
			updated.Skipped = true
		case "restore":
			updated.Skipped = false
		case "delete":
			updated.Deleted = true
		}
		if updated == current.value {
			return nil
		}
		updated.UpdatedAt = uc.now().UTC().Unix()
		updated.IsOverride = true
		if !current.persisted {
			id, err := virtualOccurrenceID(uc.issueID)
			if err != nil {
				return err
			}
			updated.ID = &id
			_, err = repos.Blueprints().MaterializeOccurrence(ctx, actorID, updated, capability)
			if err != nil {
				return mapOccurrenceConflict(err)
			}
		} else {
			_, err = repos.Blueprints().UpdatePersistedOccurrence(ctx, actorID, updated, expectedRevision, capability)
			if err != nil {
				return mapOccurrenceConflict(err)
			}
		}
		return finishScheduleProjectMutation(ctx, repos, domain.UserID(actorID), before, uc.now().UTC())
	})
	if err == nil {
		logScheduleStateChange(uc.logger, ctx, "schedule.occurrence."+uc.state, domain.UserID(actorID), domain.ScheduleID(blueprintID))
	}
	return err
}

func occurrenceStateCapability(state string) (shared.Capability, error) {
	switch state {
	case "complete", "reopen":
		return shared.ScheduleUpdate(), nil
	case "skip", "restore":
		return shared.OccurrenceUpdate(), nil
	case "delete":
		return shared.ScheduleDelete(), nil
	default:
		return shared.Capability{}, errors.New("unknown occurrence state")
	}
}

func validateOccurrenceState(current dao.ScheduleOccurrence, state string) error {
	switch state {
	case "complete":
		if current.Deleted || current.Skipped {
			return ErrOccurrenceInactive
		}
	case "reopen":
		if current.Deleted || current.Skipped {
			return ErrOccurrenceInactive
		}
	case "skip":
		if current.Deleted || current.Skipped {
			return ErrOccurrenceInactive
		}
		if current.Completed {
			return ErrOccurrenceCompleted
		}
	case "restore":
		if current.Deleted {
			return ErrOccurrenceInactive
		}
		if !current.Skipped {
			return ErrOccurrenceNotFound
		}
	case "delete":
		if current.Deleted || current.Skipped {
			return ErrOccurrenceInactive
		}
	default:
		return errors.New("unknown occurrence state")
	}
	return nil
}
