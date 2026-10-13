package usecase

import (
	"context"

	"github.com/Najah7/task2todaytodo/internal/application/shared"
	"github.com/Najah7/task2todaytodo/internal/application/todo/dao"
)

func requireSharedProjectWrite(ctx context.Context, repos Repositories, actorID, projectID string) error {
	if projectID == "" {
		return nil
	}
	readerRepos, ok := repos.(ProjectWriteAccessRepository)
	if !ok || readerRepos.ProjectWriteAccess() == nil {
		return ErrPermissionDenied
	}
	access, err := readerRepos.ProjectWriteAccess().GetProjectWriteAccess(ctx, actorID, projectID)
	if err != nil {
		return err
	}
	if !access.AllowsWrite() {
		return shared.ErrTeamSubscriptionRequired
	}
	return nil
}

// filterSharedPlanningCandidates keeps read access to shared work while ensuring
// a plan never selects work the current user cannot complete or reopen.
func filterSharedPlanningCandidates(ctx context.Context, repos Repositories, actorID string, candidates []dao.WorkSource) ([]dao.WorkSource, error) {
	readerRepos, ok := repos.(ProjectWriteAccessRepository)
	if !ok || readerRepos.ProjectWriteAccess() == nil {
		filtered := make([]dao.WorkSource, 0, len(candidates))
		for _, candidate := range candidates {
			if candidate.ProjectID == "" {
				filtered = append(filtered, candidate)
			}
		}
		return filtered, nil
	}
	reader := readerRepos.ProjectWriteAccess()
	accessByProject := make(map[string]bool)
	checked := make(map[string]bool)
	filtered := make([]dao.WorkSource, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate.ProjectID == "" {
			filtered = append(filtered, candidate)
			continue
		}
		if !checked[candidate.ProjectID] {
			access, err := reader.GetProjectWriteAccess(ctx, actorID, candidate.ProjectID)
			if err != nil {
				return nil, err
			}
			checked[candidate.ProjectID] = true
			accessByProject[candidate.ProjectID] = access.AllowsWrite()
		}
		if accessByProject[candidate.ProjectID] {
			filtered = append(filtered, candidate)
		}
	}
	return filtered, nil
}

func todoPlanAccess(ctx context.Context, repos Repositories, actorID string, fallback shared.PlanAccessReader) (shared.PlanAccess, error) {
	if readerRepos, ok := repos.(ProjectWriteAccessRepository); ok && readerRepos.ProjectWriteAccess() != nil {
		if planReader, ok := readerRepos.ProjectWriteAccess().(shared.PlanAccessReader); ok {
			return planReader.GetPlanAccess(ctx, actorID)
		}
	}
	if fallback != nil {
		return fallback.GetPlanAccess(ctx, actorID)
	}
	return shared.PlanAccessFree, nil
}

func restrictSharedWriteCapabilities(ctx context.Context, repos Repositories, actorID string, sources []dao.WorkSource, schedules []dao.ScheduleOccurrence) error {
	readerRepos, ok := repos.(ProjectWriteAccessRepository)
	var reader shared.ProjectWriteAccessReader
	if ok {
		reader = readerRepos.ProjectWriteAccess()
	}
	allowed := make(map[string]bool)
	checked := make(map[string]bool)
	canWrite := func(projectID string) (bool, error) {
		if value, ok := allowed[projectID]; ok {
			return value, nil
		}
		if checked[projectID] || reader == nil {
			return false, nil
		}
		access, err := reader.GetProjectWriteAccess(ctx, actorID, projectID)
		if err != nil {
			return false, err
		}
		checked[projectID] = true
		allowed[projectID] = access.AllowsWrite()
		return allowed[projectID], nil
	}
	for i := range sources {
		if sources[i].ProjectID == "" {
			continue
		}
		writeAllowed, err := canWrite(sources[i].ProjectID)
		if err != nil {
			return err
		}
		if !writeAllowed {
			clearWorkWriteCapabilities(&sources[i])
		}
	}
	for i := range schedules {
		if schedules[i].ProjectID == "" {
			continue
		}
		writeAllowed, err := canWrite(schedules[i].ProjectID)
		if err != nil {
			return err
		}
		if !writeAllowed {
			clearScheduleWriteCapabilities(&schedules[i])
		}
	}
	return nil
}

func clearWorkWriteCapabilities(source *dao.WorkSource) {
	source.CanComplete = false
	source.CanReopen = false
	source.CanAddToToday = false
	source.CanAddActionItem = false
	source.CanRemoveFromToday = false
	source.EstimateEditable = false
}

func clearScheduleWriteCapabilities(source *dao.ScheduleOccurrence) {
	source.CanComplete = false
	source.CanReopen = false
	source.CanHide = false
	source.CanRestore = false
}
