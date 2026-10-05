package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/shared"
	"github.com/Najah7/task2todaytodo/internal/application/task/dao"
	"github.com/Najah7/task2todaytodo/internal/application/task/domain"
)

func mustLocation(name string) *time.Location {
	location, err := time.LoadLocation(name)
	if err != nil {
		panic(err)
	}
	return location
}

type scheduleRuleUOWFake struct{ repos Repositories }

func (u scheduleRuleUOWFake) Do(ctx context.Context, fn func(context.Context, Repositories) error) error {
	return fn(ctx, u.repos)
}

type scheduleRuleReposFake struct {
	taskProgressTestRepositories
	schedules TaskScheduleRepository
	tasks     TaskRepository
}

func (r scheduleRuleReposFake) TaskSchedules() TaskScheduleRepository { return r.schedules }
func (r scheduleRuleReposFake) Tasks() TaskRepository {
	if r.tasks != nil {
		return r.tasks
	}
	return taskProgressTestRepository{}
}

type scheduleRuleRepositoryFake struct {
	TaskScheduleRepository
	root        dao.TaskSchedule
	anchor      time.Time
	interval    int
	frequencies []dao.TaskFrequency
	sets, stops int
}

func (r *scheduleRuleRepositoryFake) GetByTaskAndUserID(context.Context, domain.UserID, domain.TaskID, domain.TaskScheduleID) (dao.TaskSchedule, error) {
	return r.root, nil
}
func (r *scheduleRuleRepositoryFake) SetTaskScheduleRecurrence(_ context.Context, _ domain.UserID, _ domain.TaskID, _ domain.TaskScheduleID, anchor time.Time, interval int, frequencies []dao.TaskFrequency) error {
	r.anchor, r.interval, r.frequencies, r.sets = anchor, interval, frequencies, r.sets+1
	r.root.FrequencyAnchorDate = anchor.Unix()
	r.root.IntervalWeeks = interval
	r.root.Frequencies = frequencies
	r.root.RepeatState = repeatStateActive
	return nil
}
func (r *scheduleRuleRepositoryFake) StopTaskScheduleRecurrence(context.Context, domain.UserID, domain.TaskID, domain.TaskScheduleID) error {
	r.stops++
	r.root.RepeatState = repeatStateStopped
	r.root.IntervalWeeks = 0
	return nil
}

type scheduleFrequencyProgressFake struct {
	taskProgressTestRepository
	schedules *scheduleRuleRepositoryFake
	reads     []time.Time
	writes    int
}

func (r *scheduleFrequencyProgressFake) LockByUserID(_ context.Context, userID domain.UserID, taskID domain.TaskID) (dao.Task, error) {
	return dao.Task{ID: string(taskID), UserID: string(userID), Status: dao.TaskStatus{Value: "done"}}, nil
}
func (r *scheduleFrequencyProgressFake) LockByUserIDWithPermission(ctx context.Context, userID domain.UserID, taskID domain.TaskID, _ shared.Capability) (dao.Task, error) {
	return r.LockByUserID(ctx, userID, taskID)
}
func (r *scheduleFrequencyProgressFake) SetStatusByUserID(_ context.Context, _ domain.UserID, _ domain.TaskID, status domain.TaskStatus) error {
	r.writes++
	if status.Value != "open" {
		return errors.New("expected task to reopen")
	}
	return nil
}
func (r *scheduleFrequencyProgressFake) SetStatusByUserIDWithPermission(ctx context.Context, userID domain.UserID, taskID domain.TaskID, status domain.TaskStatus, _ int32, _ shared.Capability) error {
	return r.SetStatusByUserID(ctx, userID, taskID, status)
}
func (r *scheduleFrequencyProgressFake) ReadTaskProgressSources(_ context.Context, taskIDs, _ []string, asOf time.Time) (dao.TaskProgressSources, error) {
	r.reads = append(r.reads, asOf)
	root := r.schedules.root
	return dao.TaskProgressSources{
		Counts:   map[string]dao.TaskProgressCounts{},
		Statuses: map[string]dao.TaskStatus{string(taskIDs[0]): {Value: "done"}},
		TaskScheduleRoots: []dao.ProgressRecurrence{{
			TaskID: root.TaskID, SeriesID: root.SeriesID, OccurrenceDate: root.OccurrenceDate, Timezone: root.Timezone,
			RepeatState: root.RepeatState, FrequencyAnchorDate: root.FrequencyAnchorDate,
			IntervalWeeks: root.IntervalWeeks, Frequencies: root.Frequencies, StartAt: root.StartAt, EndAt: root.EndAt,
		}},
	}, nil
}

func TestUpdateTaskScheduleFrequencyAnchorsLocalTodayWithoutResolvingDSTGap(t *testing.T) {
	loc := mustLocation("America/Los_Angeles")
	start := time.Date(2026, 1, 5, 2, 30, 0, 0, loc)
	end := start.Add(time.Hour)
	rootDate := mustParseDate("2026-01-05")
	repo := &scheduleRuleRepositoryFake{root: dao.TaskSchedule{ID: "schedule-1", TaskID: "task-1", SeriesID: "schedule-1", OccurrenceDate: rootDate.Format("2006-01-02"), Timezone: loc.String(), Title: "Focus", IntervalWeeks: 1, Frequencies: []dao.TaskFrequency{{Value: "mon"}}, StartAt: start.Unix(), EndAt: end.Unix(), RepeatState: repeatStateActive}}
	uc := NewUpdateTaskScheduleFrequencyUseCase(scheduleRuleUOWFake{repos: scheduleRuleReposFake{schedules: repo}}, nil)
	uc.now = func() time.Time { return time.Date(2026, 3, 8, 10, 0, 0, 0, time.UTC) }
	err := uc.Execute(context.Background(), UpdateTaskScheduleFrequencyInput{UserID: "user-1", TaskID: "task-1", TaskScheduleID: "schedule-1", IntervalWeeks: 2, Frequencies: []string{"wed"}})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := repo.anchor.Format("2006-01-02"), "2026-03-08"; got != want {
		t.Fatalf("anchor=%s want %s", got, want)
	}
	if repo.sets != 1 || repo.interval != 2 || repo.root.StartAt != start.Unix() {
		t.Fatalf("sets=%d interval=%d root start changed", repo.sets, repo.interval)
	}
}

func TestUpdateTaskScheduleFrequencyReopensDoneTaskWhenTodayVirtualOccurrenceAppears(t *testing.T) {
	loc := mustLocation("UTC")
	start := time.Date(2026, 1, 5, 9, 0, 0, 0, loc)
	rootDate := mustParseDate("2026-01-05")
	repo := &scheduleRuleRepositoryFake{root: dao.TaskSchedule{ID: "schedule-1", TaskID: "task-1", SeriesID: "schedule-1", OccurrenceDate: rootDate.Format("2006-01-02"), Timezone: loc.String(), Title: "Focus", IntervalWeeks: 1, Frequencies: []dao.TaskFrequency{{Value: "tue"}}, StartAt: start.Unix(), EndAt: start.Add(time.Hour).Unix(), RepeatState: repeatStateActive}}
	progress := &scheduleFrequencyProgressFake{schedules: repo}
	fixedAsOf := time.Date(2026, 10, 5, 12, 34, 56, 123, time.UTC)
	uc := NewUpdateTaskScheduleFrequencyUseCase(scheduleRuleUOWFake{repos: scheduleRuleReposFake{schedules: repo, tasks: progress}}, nil)
	uc.now = func() time.Time { return fixedAsOf }
	err := uc.Execute(context.Background(), UpdateTaskScheduleFrequencyInput{UserID: "user-1", TaskID: "task-1", TaskScheduleID: "schedule-1", IntervalWeeks: 1, Frequencies: []string{"mon"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(progress.reads) != 2 || !progress.reads[0].Equal(fixedAsOf) || !progress.reads[1].Equal(fixedAsOf) {
		t.Fatalf("progress snapshot times=%v, want two snapshots at %v", progress.reads, fixedAsOf)
	}
	if progress.writes != 1 {
		t.Fatalf("status writes=%d, want task reopened once", progress.writes)
	}
}
