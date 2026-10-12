package repository

import (
	"context"
	"errors"
	"sort"

	"github.com/Najah7/task2todaytodo/db/sqlc"
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

func (r *ScheduleRepository) DeleteProjectSchedulesByActor(ctx context.Context, actor domain.UserID, project domain.ProjectID) error {
	_, err := r.queries.DeleteProjectRuleOccurrencesByActor(ctx, sqlc.DeleteProjectRuleOccurrencesByActorParams{ActorID: string(actor), ProjectID: string(project)})
	return err
}

func (r *ScheduleRepository) ReassignProjectMemberSchedules(ctx context.Context, actor domain.UserID, project domain.ProjectID, member domain.UserID) error {
	candidates, err := r.queries.ListProjectMemberRuleRevisionCandidates(ctx, sqlc.ListProjectMemberRuleRevisionCandidatesParams{
		ProjectID: string(project), MemberID: string(member),
	})
	if err != nil {
		return err
	}
	if len(candidates) != 0 {
		ruleSet := make(map[string]struct{}, len(candidates))
		for _, candidate := range candidates {
			ruleSet[candidate.RuleID] = struct{}{}
		}
		ruleIDs := make([]string, 0, len(ruleSet))
		for ruleID := range ruleSet {
			ruleIDs = append(ruleIDs, ruleID)
		}
		sort.Strings(ruleIDs)
		for _, ruleID := range ruleIDs {
			if _, err := r.queries.LockScheduleRuleForMemberReassignment(ctx, ruleID); err != nil {
				return err
			}
		}
		lockedCandidates, err := r.queries.ListProjectMemberRuleRevisionCandidates(ctx, sqlc.ListProjectMemberRuleRevisionCandidatesParams{
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
				candidates[i].RuleID != lockedCandidates[i].RuleID ||
				candidates[i].TargetEffectiveFrom != lockedCandidates[i].TargetEffectiveFrom {
				return usecase.ErrScheduleProjectChanged
			}
		}
		candidates = lockedCandidates
		ids := make([]string, len(candidates))
		for i := range ids {
			ids[i] = ulid.Make().String()
		}
		updated, err := r.queries.ReassignProjectMemberRuleRevisions(ctx, sqlc.ReassignProjectMemberRuleRevisionsParams{
			ActorID: string(actor), ProjectID: string(project), MemberID: string(member), RevisionIds: ids,
		})
		if err != nil {
			return err
		}
		if int(updated) != len(candidates) {
			return usecase.ErrScheduleProjectChanged
		}
	}
	_, err = r.queries.ReassignProjectMemberRuleOccurrencesByActor(ctx, sqlc.ReassignProjectMemberRuleOccurrencesByActorParams{ActorID: string(actor), ProjectID: string(project), MemberID: string(member)})
	return err
}
