package usecase

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/schedule/dao"
	"github.com/Najah7/task2todaytodo/internal/application/shared"
)

type resolvedOccurrence struct {
	value     dao.ScheduleOccurrence
	persisted bool
}

// occurrenceForDate resolves a readable concrete row first, then projects the
// effective recurrence rule. A moved occurrence is found by its blueprint/date
// key instead of by the occurrence's current interval.
func occurrenceForDate(ctx context.Context, repo BlueprintRepository, actorID, blueprintID, occurrenceDate string) (dao.ScheduleOccurrence, error) {
	if _, err := time.Parse("2006-01-02", occurrenceDate); err != nil {
		return dao.ScheduleOccurrence{}, err
	}
	concrete, err := repo.GetPersistedOccurrence(ctx, actorID, blueprintID, occurrenceDate)
	if err == nil {
		return concrete, nil
	}
	if !errors.Is(err, ErrOccurrenceNotFound) {
		return dao.ScheduleOccurrence{}, err
	}
	resolved, err := virtualOccurrenceForDate(ctx, repo, actorID, blueprintID, occurrenceDate, shared.ScheduleRead())
	return resolved, err
}

func resolveOccurrenceForMutation(ctx context.Context, repo BlueprintRepository, actorID, blueprintID, occurrenceDate string, capability shared.Capability, includeInactive bool) (resolvedOccurrence, error) {
	if _, err := time.Parse("2006-01-02", occurrenceDate); err != nil {
		return resolvedOccurrence{}, err
	}
	concrete, err := repo.GetPersistedOccurrenceForMutation(ctx, actorID, blueprintID, occurrenceDate, capability)
	if err == nil {
		if !includeInactive && (concrete.Deleted || concrete.Skipped) {
			return resolvedOccurrence{}, ErrOccurrenceInactive
		}
		return resolvedOccurrence{value: concrete, persisted: true}, nil
	}
	if !errors.Is(err, ErrOccurrenceNotFound) {
		return resolvedOccurrence{}, err
	}
	virtual, err := virtualOccurrenceForDate(ctx, repo, actorID, blueprintID, occurrenceDate, capability)
	if err != nil {
		return resolvedOccurrence{}, err
	}
	return resolvedOccurrence{value: virtual}, nil
}

func lockAndReloadOccurrence(ctx context.Context, repos Repositories, actorID, blueprintID, occurrenceDate string, initial resolvedOccurrence, expectedRevision int32, capability shared.Capability, projectIDs []string, asOf time.Time, includeInactive bool) (resolvedOccurrence, scheduleProjectMutationSnapshots, error) {
	if initial.value.Revision != expectedRevision {
		return resolvedOccurrence{}, nil, ErrOccurrenceRevisionMismatch
	}
	before, err := lockAndCaptureScheduleProjects(ctx, repos, projectIDs, asOf)
	if err != nil {
		return resolvedOccurrence{}, nil, err
	}
	if err := repos.Blueprints().LockOccurrenceForMutation(ctx, actorID, blueprintID, occurrenceDate, capability); err != nil {
		return resolvedOccurrence{}, nil, err
	}
	current, err := resolveOccurrenceForMutation(ctx, repos.Blueprints(), actorID, blueprintID, occurrenceDate, capability, includeInactive)
	if err != nil {
		return resolvedOccurrence{}, nil, err
	}
	if current.value.Revision != expectedRevision || current.persisted != initial.persisted ||
		current.value.ProjectID != initial.value.ProjectID || current.value.AssigneeID != initial.value.AssigneeID ||
		current.value.SourceRevisionID != initial.value.SourceRevisionID {
		return resolvedOccurrence{}, nil, ErrOccurrenceRevisionMismatch
	}
	return current, before, nil
}

func virtualOccurrenceID(issueID func() string) (string, error) {
	if issueID == nil {
		return "", ErrOccurrenceIDGeneratorUnavailable
	}
	id := issueID()
	if strings.TrimSpace(id) == "" {
		return "", ErrOccurrenceIDGeneratorUnavailable
	}
	return id, nil
}

func virtualOccurrenceForDate(ctx context.Context, repo BlueprintRepository, actorID, blueprintID, occurrenceDate string, capability shared.Capability) (dao.ScheduleOccurrence, error) {
	date, err := time.Parse("2006-01-02", occurrenceDate)
	if err != nil {
		return dao.ScheduleOccurrence{}, err
	}
	revision, err := repo.GetEffectiveRevisionForDate(ctx, actorID, blueprintID, occurrenceDate, capability)
	if err != nil {
		return dao.ScheduleOccurrence{}, err
	}
	pauses, err := repo.ListPauseIntervals(ctx, actorID, blueprintID)
	if err != nil {
		return dao.ScheduleOccurrence{}, err
	}
	revision.PauseIntervals = pauses
	// Include adjacent UTC days so timezone offsets and cross-midnight events
	// still project the requested local occurrence date.
	from := date.AddDate(0, 0, -2).UTC()
	to := date.AddDate(0, 0, 3).UTC()
	origins, err := repo.ListOccurrenceOrigins(ctx, actorID, []string{blueprintID}, from, to)
	if err != nil {
		return dao.ScheduleOccurrence{}, err
	}
	items, err := projectOccurrences([]dao.BlueprintRevision{revision}, nil, origins, from, to, false)
	if err != nil {
		return dao.ScheduleOccurrence{}, err
	}
	for _, item := range items {
		if item.BlueprintID == blueprintID && item.OccurrenceDate == occurrenceDate {
			return item, nil
		}
	}
	return dao.ScheduleOccurrence{}, ErrOccurrenceNotFound
}

func intPointer(value int32) *int32 { return &value }
