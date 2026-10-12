package usecase

import (
	"context"
	"time"
)

type ProjectPauseRule struct {
	RuleID string
	UserID string
}

// ProjectLifecycleRepository is the transaction-bound persistence surface
// used while a Project is completed or reopened.
type ProjectLifecycleRepository interface {
	ListProjectAllocationUsers(context.Context, string) ([]string, error)
	LockProjectAllocationUser(context.Context, string) error
	PauseProjectAllocations(context.Context, string, time.Time) ([]string, error)
	ListProjectAllocationPauseRulesForResume(context.Context, string) ([]ProjectPauseRule, error)
	ResumeProjectAllocationRule(context.Context, string, string, string, time.Time) error
	PauseProjectAllocationRuleForConflict(context.Context, string, string, string, time.Time) error
	CloseProjectAllocationPauseInterval(context.Context, string, string, time.Time) error
}

func PauseProjectAllocations(ctx context.Context, repo ProjectLifecycleRepository, _, projectID string, at time.Time) error {
	users, err := lockProjectAllocationUsers(ctx, repo, projectID)
	if err != nil {
		return err
	}
	changedUsers, err := repo.PauseProjectAllocations(ctx, projectID, at)
	if err != nil {
		return err
	}
	locked := make(map[string]bool, len(users))
	for _, userID := range users {
		locked[userID] = true
	}
	for _, userID := range changedUsers {
		if !locked[userID] {
			return ErrPreconditionFailed
		}
	}
	return nil
}

func lockProjectAllocationUsers(ctx context.Context, repo ProjectLifecycleRepository, projectID string) ([]string, error) {
	users, err := repo.ListProjectAllocationUsers(ctx, projectID)
	if err != nil {
		return nil, err
	}
	for _, userID := range users {
		if err := repo.LockProjectAllocationUser(ctx, userID); err != nil {
			return nil, err
		}
	}
	return users, nil
}

// ResumeProjectAllocations revalidates each paused rule independently. A
// conflicting rule stays paused with a stable reason while compatible rules
// resume, all within the Project transition transaction.
func ResumeProjectAllocations(ctx context.Context, repo Repository, lifecycle ProjectLifecycleRepository, actorID, projectID string, at time.Time) error {
	if _, err := lockProjectAllocationUsers(ctx, lifecycle, projectID); err != nil {
		return err
	}
	rules, err := lifecycle.ListProjectAllocationPauseRulesForResume(ctx, projectID)
	if err != nil {
		return err
	}
	for _, rule := range rules {
		reason, err := ValidateProjectResume(ctx, repo, rule.UserID, rule.RuleID, projectID, at)
		if err != nil {
			if err == ErrNotFound {
				// The rule's latest revision moved to another Project after this
				// pause began. Close the old Project interval, leave the rule
				// paused, and let reopening the former Project succeed.
				if err := lifecycle.PauseProjectAllocationRuleForConflict(ctx, rule.UserID, rule.RuleID, "allocation_conflict", at); err != nil {
					return err
				}
				if err := lifecycle.CloseProjectAllocationPauseInterval(ctx, projectID, rule.RuleID, at); err != nil {
					return err
				}
				continue
			}
			return err
		}
		if reason == "" {
			if err := lifecycle.ResumeProjectAllocationRule(ctx, rule.UserID, rule.RuleID, projectID, at); err != nil {
				return err
			}
			continue
		}
		if err := lifecycle.PauseProjectAllocationRuleForConflict(ctx, rule.UserID, rule.RuleID, reason, at); err != nil {
			return err
		}
		if err := lifecycle.CloseProjectAllocationPauseInterval(ctx, projectID, rule.RuleID, at); err != nil {
			return err
		}
	}
	return nil
}
