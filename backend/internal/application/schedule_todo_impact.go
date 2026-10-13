package application

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/Najah7/task2todaytodo/db/sqlc"
	"github.com/Najah7/task2todaytodo/internal/application/schedule/usecase"
	tododao "github.com/Najah7/task2todaytodo/internal/application/todo/dao"
	todousecase "github.com/Najah7/task2todaytodo/internal/application/todo/usecase"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type todoScheduleSnapshot struct {
	userID    string
	todo      tododao.Todo
	hasList   bool
	schedules []tododao.ScheduleOccurrence
}

func (u *ScheduleUOW) DoWithTodoImpact(ctx context.Context, scope usecase.TodoImpactScope, fn func(context.Context, usecase.Repositories) error) error {
	return RunInTodoWorkflowTx(ctx, u.pool, func(tx pgx.Tx) error {
		targets, err := scheduleImpactUsers(ctx, tx, scope)
		if err != nil {
			return err
		}
		if scope.AssigneeID != "" {
			targets = append(targets, scope.AssigneeID)
		}
		targets = uniqueSortedStrings(targets)

		repos := u.repositories(tx)
		todoRepos := newTodoRepositories(u.sourceStore, u.taskStore, u.projects, u.store, tx, u.newID, nil)
		for _, userID := range targets {
			if err := todoRepos.Todos().LockUser(ctx, userID); err != nil {
				return err
			}
		}
		before, err := captureTodoScheduleSnapshots(ctx, todoRepos, targets)
		if err != nil {
			return err
		}
		if err := fn(ctx, repos); err != nil {
			return err
		}
		after, err := captureTodoScheduleSnapshots(ctx, todoRepos, targets)
		if err != nil {
			return err
		}
		changed := make([]string, 0, len(targets))
		for _, userID := range targets {
			oldSnapshot, oldOK := before[userID]
			newSnapshot, newOK := after[userID]
			if oldOK && newOK && oldSnapshot.hasList && newSnapshot.hasList && !sameTodoScheduleSources(oldSnapshot.schedules, newSnapshot.schedules) {
				changed = append(changed, userID)
			}
		}
		if len(changed) == 0 {
			return nil
		}
		for _, userID := range changed {
			if err := syncTodoSchedules(ctx, todoRepos, before[userID], after[userID], u.newID); err != nil {
				return err
			}
		}
		return nil
	})
}

func scheduleImpactUsers(ctx context.Context, tx pgx.Tx, scope usecase.TodoImpactScope) ([]string, error) {
	var date pgtype.Date
	if scope.OccurrenceDate != "" {
		parsed, err := time.Parse("2006-01-02", scope.OccurrenceDate)
		if err != nil {
			return nil, fmt.Errorf("parse Todo impact occurrence date: %w", err)
		}
		date = pgtype.Date{Time: parsed, Valid: true}
	}
	assignee := pgtype.Text{String: scope.AssigneeID, Valid: scope.AssigneeID != ""}
	project := pgtype.Text{String: scope.ProjectID, Valid: scope.ProjectID != ""}
	users, err := sqlc.New(tx).ListScheduleTodoImpactUsers(ctx, sqlc.ListScheduleTodoImpactUsersParams{
		RuleID: scope.RuleID, OccurrenceDate: date, Following: scope.Following,
		AssigneeID: assignee, ActorID: scope.ActorID, ProjectID: project,
	})
	if err != nil {
		return nil, err
	}
	return users, nil
}

func captureTodoScheduleSnapshots(ctx context.Context, repos todousecase.Repositories, userIDs []string) (map[string]todoScheduleSnapshot, error) {
	result := make(map[string]todoScheduleSnapshot, len(userIDs))
	for _, userID := range userIDs {
		day, err := repos.Sources().CurrentDay(ctx, userID)
		if err != nil {
			return nil, err
		}
		snapshot := todoScheduleSnapshot{userID: userID}
		todo, err := repos.Todos().GetByUserDate(ctx, userID, day.Date)
		if err == todousecase.ErrNotFound {
			result[userID] = snapshot
			continue
		}
		if err != nil {
			return nil, err
		}
		snapshot.hasList, snapshot.todo = true, todo
		sources, err := repos.Sources().ReadPlanningSources(ctx, userID, day.Date, day.Timezone)
		if err != nil {
			return nil, err
		}
		snapshot.schedules = sources.Schedules
		result[userID] = snapshot
	}
	return result, nil
}

func sameTodoScheduleSources(left, right []tododao.ScheduleOccurrence) bool {
	if len(left) != len(right) {
		return false
	}
	left = append([]tododao.ScheduleOccurrence(nil), left...)
	right = append([]tododao.ScheduleOccurrence(nil), right...)
	sortTodoScheduleSources(left)
	sortTodoScheduleSources(right)
	for i := range left {
		if scheduleTodoKey(left[i].RuleID, left[i].OccurrenceDate) != scheduleTodoKey(right[i].RuleID, right[i].OccurrenceDate) ||
			!left[i].StartAt.Equal(right[i].StartAt) || !left[i].EndAt.Equal(right[i].EndAt) || left[i].AllDay != right[i].AllDay {
			return false
		}
	}
	return true
}

func sortTodoScheduleSources(rows []tododao.ScheduleOccurrence) {
	sort.Slice(rows, func(i, j int) bool {
		if !rows[i].StartAt.Equal(rows[j].StartAt) {
			return rows[i].StartAt.Before(rows[j].StartAt)
		}
		if rows[i].RuleID != rows[j].RuleID {
			return rows[i].RuleID < rows[j].RuleID
		}
		return rows[i].OccurrenceDate.Before(rows[j].OccurrenceDate)
	})
}

func syncTodoSchedules(ctx context.Context, repos todousecase.Repositories, before, after todoScheduleSnapshot, newID func() string) error {
	return todousecase.SyncScheduleSourcesInTx(ctx, repos, after.todo, after.schedules, newID)
}

func scheduleTodoKey(ruleID string, occurrenceDate time.Time) string {
	return ruleID + "/" + occurrenceDate.Format("2006-01-02")
}

func uniqueSortedStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}
