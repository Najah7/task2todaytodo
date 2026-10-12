package repository

import (
	"context"
	"time"

	"github.com/Najah7/task2todaytodo/db/sqlc"
	allocationusecase "github.com/Najah7/task2todaytodo/internal/application/allocation/usecase"
	"github.com/jackc/pgx/v5/pgtype"
)

// LockProjectAllocationUsers acquires the allocation locks after the caller
// has locked the Project row. The query returns stable sorted user IDs so
// Project transitions and ordinary allocation writes share one lock order.
func (r *AllocationRepository) LockProjectAllocationUsers(ctx context.Context, projectID string) ([]string, error) {
	users, err := r.ListProjectAllocationUsers(ctx, projectID)
	if err != nil {
		return nil, err
	}
	for _, userID := range users {
		if err := r.LockProjectAllocationUser(ctx, userID); err != nil {
			return nil, err
		}
	}
	return users, nil
}

func (r *AllocationRepository) ListProjectAllocationUsers(ctx context.Context, projectID string) ([]string, error) {
	return r.queries.ListProjectAllocationUsersForLifecycle(ctx, projectID)
}

func (r *AllocationRepository) LockProjectAllocationUser(ctx context.Context, userID string) error {
	return r.LockUser(ctx, userID)
}

func (r *AllocationRepository) PauseProjectAllocations(ctx context.Context, projectID string, at time.Time) ([]string, error) {
	atTime := pgInstant(at)
	rows, err := r.queries.PauseProjectAllocationRulesForLifecycle(ctx, sqlc.PauseProjectAllocationRulesForLifecycleParams{
		ProjectID: projectID, PausedAt: atTime,
	})
	if err != nil {
		return nil, err
	}
	if err := r.queries.StartProjectAllocationPauseIntervalsForLifecycle(ctx, sqlc.StartProjectAllocationPauseIntervalsForLifecycleParams{
		PausedAt: atTime, ProjectID: projectID,
	}); err != nil {
		return nil, err
	}
	return rows, nil
}

func (r *AllocationRepository) ListProjectAllocationPauseRulesForResume(ctx context.Context, projectID string) ([]allocationusecase.ProjectPauseRule, error) {
	rows, err := r.queries.ListProjectAllocationPauseRulesForResume(ctx, projectID)
	if err != nil {
		return nil, err
	}
	rules := make([]allocationusecase.ProjectPauseRule, 0, len(rows))
	for _, row := range rows {
		rules = append(rules, allocationusecase.ProjectPauseRule{RuleID: row.RuleID, UserID: row.UserID})
	}
	return rules, nil
}

func (r *AllocationRepository) ResumeProjectAllocationRule(ctx context.Context, userID, ruleID, projectID string, resumedAt time.Time) error {
	if err := r.queries.ResumeProjectAllocationRule(ctx, sqlc.ResumeProjectAllocationRuleParams{RuleID: ruleID, UserID: userID}); err != nil {
		return err
	}
	return r.queries.CloseProjectAllocationPauseInterval(ctx, sqlc.CloseProjectAllocationPauseIntervalParams{
		ProjectID: projectID, RuleID: ruleID, ResumedAt: pgInstant(resumedAt),
	})
}

func (r *AllocationRepository) PauseProjectAllocationRuleForConflict(ctx context.Context, userID, ruleID, reason string, at time.Time) error {
	return r.queries.SetAllocationPause(ctx, sqlc.SetAllocationPauseParams{
		RuleID: ruleID, UserID: userID,
		PauseReason: pgtype.Text{String: reason, Valid: true}, PausedAt: pgInstant(at),
	})
}

func (r *AllocationRepository) CloseProjectAllocationPauseInterval(ctx context.Context, projectID, ruleID string, at time.Time) error {
	return r.queries.CloseProjectAllocationPauseInterval(ctx, sqlc.CloseProjectAllocationPauseIntervalParams{
		ProjectID: projectID, RuleID: ruleID, ResumedAt: pgInstant(at),
	})
}

var _ allocationusecase.ProjectLifecycleRepository = (*AllocationRepository)(nil)
