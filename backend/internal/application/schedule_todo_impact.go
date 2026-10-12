package application

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/Najah7/task2todaytodo/db/sqlc"
	"github.com/Najah7/task2todaytodo/internal/application/schedule/usecase"
	todousecase "github.com/Najah7/task2todaytodo/internal/application/todo/usecase"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type todoScheduleReservation struct {
	ruleID, occurrenceDate string
	start, end             time.Time
	allDay                 bool
}

type todoScheduleSnapshot struct {
	userID  string
	listDay time.Time
	hasList bool
	rows    []todoScheduleReservation
}

func (u *ScheduleUOW) DoWithTodoImpact(ctx context.Context, scope usecase.TodoImpactScope, confirmed bool, fn func(context.Context, usecase.Repositories) error) error {
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
			if oldOK && newOK && oldSnapshot.hasList && newSnapshot.hasList && !sameTodoReservations(oldSnapshot.rows, newSnapshot.rows) {
				changed = append(changed, userID)
			}
		}
		if len(changed) == 0 {
			return nil
		}
		if !confirmed {
			return usecase.ErrTodoRegenerationRequired
		}
		regenerator := todousecase.NewRegenerateTodayUseCase(nil, u.newID)
		for _, userID := range changed {
			snapshot := after[userID]
			if err := todoRepos.Todos().LockUser(ctx, userID); err != nil {
				return err
			}
			if _, err := regenerator.RegenerateTodayInTx(ctx, userID, snapshot.listDay, todoRepos); err != nil {
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
		snapshot := todoScheduleSnapshot{userID: userID, listDay: day.Date}
		_, err = repos.Todos().GetByUserDate(ctx, userID, day.Date)
		if err == todousecase.ErrNotFound {
			result[userID] = snapshot
			continue
		}
		if err != nil {
			return nil, err
		}
		snapshot.hasList = true
		sources, err := repos.Sources().ReadPlanningSources(ctx, userID, day.Date, day.Timezone)
		if err != nil {
			return nil, err
		}
		snapshot.rows = make([]todoScheduleReservation, 0, len(sources.Schedules))
		for _, schedule := range sources.Schedules {
			snapshot.rows = append(snapshot.rows, todoScheduleReservation{
				ruleID: schedule.RuleID, occurrenceDate: schedule.OccurrenceDate.Format("2006-01-02"),
				start: schedule.StartAt, end: schedule.EndAt, allDay: schedule.AllDay,
			})
		}
		sortTodoReservations(snapshot.rows)
		result[userID] = snapshot
	}
	return result, nil
}

func sameTodoReservations(left, right []todoScheduleReservation) bool {
	if len(left) != len(right) {
		return false
	}
	sortTodoReservations(left)
	sortTodoReservations(right)
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

func sortTodoReservations(rows []todoScheduleReservation) {
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].start != rows[j].start {
			return rows[i].start.Before(rows[j].start)
		}
		if rows[i].ruleID != rows[j].ruleID {
			return rows[i].ruleID < rows[j].ruleID
		}
		return rows[i].occurrenceDate < rows[j].occurrenceDate
	})
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
