package repository

import (
	"context"
	"errors"
	"sort"

	"github.com/Najah7/task2todaytodo/db/sqlc"
	"github.com/Najah7/task2todaytodo/internal/application/schedule/dao"
	"github.com/Najah7/task2todaytodo/internal/application/schedule/domain"
	"github.com/Najah7/task2todaytodo/internal/application/schedule/usecase"
	"github.com/Najah7/task2todaytodo/internal/application/shared"
	"github.com/jackc/pgx/v5"
	"github.com/oklog/ulid/v2"
)

var _ usecase.ScheduleRepository = (*ScheduleRepository)(nil)

type ScheduleRepository struct{ queries *sqlc.Queries }

func NewScheduleRepository(db sqlc.DBTX) *ScheduleRepository {
	return &ScheduleRepository{queries: sqlc.New(db)}
}

func (r *ScheduleRepository) WithTx(tx pgx.Tx) *ScheduleRepository {
	return &ScheduleRepository{queries: r.queries.WithTx(tx)}
}

func (r *ScheduleRepository) LockProjectForScheduleMutation(ctx context.Context, projectID domain.ProjectID) error {
	if projectID == "" {
		return nil
	}
	if _, err := r.queries.LockProjectForLifecycle(ctx, string(projectID)); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return usecase.ErrScheduleProjectNotFound
		}
		return err
	}
	return nil
}

func (r *ScheduleRepository) CheckProjectPermission(ctx context.Context, actor domain.UserID, projectID domain.ProjectID, capability shared.Capability) (bool, error) {
	allowed, err := r.queries.HasProjectPermission(ctx, sqlc.HasProjectPermissionParams{
		ProjectID: string(projectID), ActorID: string(actor), ResourceID: string(capability.Resource), Action: sqlc.Action(capability.Action),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return false, usecase.ErrScheduleProjectNotFound
	}
	return allowed, err
}

func (r *ScheduleRepository) ListTags(ctx context.Context, actor domain.UserID, id domain.ScheduleID) ([]dao.ScheduleTag, error) {
	rows, err := r.queries.ListScheduleTagsByScheduleAndUserID(ctx, sqlc.ListScheduleTagsByScheduleAndUserIDParams{ScheduleID: string(id), UserID: string(actor)})
	if err != nil {
		return nil, err
	}
	out := make([]dao.ScheduleTag, 0, len(rows))
	for _, row := range rows {
		out = append(out, dao.ScheduleTag{ID: row.ID, UserID: row.UserID, Name: row.Name, CreatedAt: unix(row.CreatedAt), UpdatedAt: unix(row.UpdatedAt)})
	}
	return out, nil
}

func (r *ScheduleRepository) AddTag(ctx context.Context, actor domain.UserID, id domain.ScheduleID, tagID string) error {
	result, err := r.queries.AddScheduleTagToScheduleByUserID(ctx, sqlc.AddScheduleTagToScheduleByUserIDParams{ScheduleID: string(id), TagID: tagID, UserID: string(actor)})
	if err != nil {
		return err
	}
	if !result.Owned {
		return usecase.ErrScheduleTagNotFound
	}
	return nil
}

func (r *ScheduleRepository) RemoveTag(ctx context.Context, actor domain.UserID, id domain.ScheduleID, tagID string) error {
	result, err := r.queries.RemoveScheduleTagFromScheduleByUserID(ctx, sqlc.RemoveScheduleTagFromScheduleByUserIDParams{ScheduleID: string(id), TagID: tagID, UserID: string(actor)})
	if err != nil {
		return err
	}
	if !result.Owned {
		return usecase.ErrScheduleTagNotFound
	}
	return nil
}

func (r *ScheduleRepository) DeleteProjectSchedulesByActor(ctx context.Context, actor domain.UserID, project domain.ProjectID) error {
	_, err := r.queries.DeleteProjectBlueprintOccurrencesByActor(ctx, sqlc.DeleteProjectBlueprintOccurrencesByActorParams{ActorID: string(actor), ProjectID: string(project)})
	return err
}

func (r *ScheduleRepository) ReassignProjectMemberSchedules(ctx context.Context, actor domain.UserID, project domain.ProjectID, member domain.UserID) error {
	candidates, err := r.queries.ListProjectMemberBlueprintRevisionCandidates(ctx, sqlc.ListProjectMemberBlueprintRevisionCandidatesParams{
		ProjectID: string(project), MemberID: string(member),
	})
	if err != nil {
		return err
	}
	if len(candidates) != 0 {
		blueprintSet := make(map[string]struct{}, len(candidates))
		for _, candidate := range candidates {
			blueprintSet[candidate.BlueprintID] = struct{}{}
		}
		blueprintIDs := make([]string, 0, len(blueprintSet))
		for blueprintID := range blueprintSet {
			blueprintIDs = append(blueprintIDs, blueprintID)
		}
		sort.Strings(blueprintIDs)
		for _, blueprintID := range blueprintIDs {
			if _, err := r.queries.LockScheduleBlueprintForMemberReassignment(ctx, blueprintID); err != nil {
				return err
			}
		}
		lockedCandidates, err := r.queries.ListProjectMemberBlueprintRevisionCandidates(ctx, sqlc.ListProjectMemberBlueprintRevisionCandidatesParams{
			ProjectID: string(project), MemberID: string(member),
		})
		if err != nil {
			return err
		}
		if len(lockedCandidates) != len(candidates) {
			return usecase.ErrScheduleProjectChanged
		}
		for i := range candidates {
			if candidates[i].SourceRevisionID != lockedCandidates[i].SourceRevisionID ||
				candidates[i].BlueprintID != lockedCandidates[i].BlueprintID ||
				candidates[i].TargetEffectiveFrom != lockedCandidates[i].TargetEffectiveFrom {
				return usecase.ErrScheduleProjectChanged
			}
		}
		candidates = lockedCandidates
		ids := make([]string, len(candidates))
		for i := range ids {
			ids[i] = ulid.Make().String()
		}
		updated, err := r.queries.ReassignProjectMemberBlueprintRevisions(ctx, sqlc.ReassignProjectMemberBlueprintRevisionsParams{
			ActorID: string(actor), ProjectID: string(project), MemberID: string(member), RevisionIds: ids,
		})
		if err != nil {
			return err
		}
		if int(updated) != len(candidates) {
			return usecase.ErrScheduleProjectChanged
		}
	}
	_, err = r.queries.ReassignProjectMemberBlueprintOccurrencesByActor(ctx, sqlc.ReassignProjectMemberBlueprintOccurrencesByActorParams{ActorID: string(actor), ProjectID: string(project), MemberID: string(member)})
	return err
}
