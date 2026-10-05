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

type rescheduleProgressID struct{}

func (rescheduleProgressID) Generate() string { return "saved-schedule" }

type rescheduleProgressUOW struct{ repos Repositories }

func (u rescheduleProgressUOW) Do(ctx context.Context, fn func(context.Context, Repositories) error) error {
	return fn(ctx, u.repos)
}

type rescheduleProgressRepositories struct {
	taskProgressTestRepositories
	schedules TaskScheduleRepository
	tasks     TaskRepository
}

func (r rescheduleProgressRepositories) TaskSchedules() TaskScheduleRepository { return r.schedules }
func (r rescheduleProgressRepositories) Tasks() TaskRepository                 { return r.tasks }

type rescheduleProgressScheduleRepository struct {
	TaskScheduleRepository
	root    dao.TaskSchedule
	writes  int
	updated dao.TaskSchedule
}

func (r *rescheduleProgressScheduleRepository) ListByTaskAndUserID(context.Context, domain.UserID, domain.TaskID) ([]dao.TaskSchedule, error) {
	return []dao.TaskSchedule{r.root}, nil
}
func (r *rescheduleProgressScheduleRepository) UpsertTaskScheduleOverride(_ context.Context, _ domain.UserID, schedule domain.TaskSchedule) (string, error) {
	r.writes++
	r.updated = dao.TaskSchedule{ID: "saved-schedule", TaskID: string(schedule.TaskID), SeriesID: string(schedule.SeriesID), OccurrenceDate: schedule.OccurrenceDate.Format("2006-01-02"), StartAt: schedule.StartAt.Unix(), EndAt: schedule.EndAt.Unix()}
	return r.updated.ID, nil
}

type rescheduleProgressTaskRepository struct {
	taskProgressTestRepository
	reads  []time.Time
	writes int
	counts []dao.TaskProgressCounts
}

func (r *rescheduleProgressTaskRepository) LockByUserID(_ context.Context, userID domain.UserID, taskID domain.TaskID) (dao.Task, error) {
	return dao.Task{ID: string(taskID), UserID: string(userID), Status: dao.TaskStatus{Value: "done"}}, nil
}
func (r *rescheduleProgressTaskRepository) LockByUserIDWithPermission(ctx context.Context, userID domain.UserID, taskID domain.TaskID, _ shared.Capability) (dao.Task, error) {
	return r.LockByUserID(ctx, userID, taskID)
}
func (r *rescheduleProgressTaskRepository) SetStatusByUserID(_ context.Context, _ domain.UserID, _ domain.TaskID, status domain.TaskStatus) error {
	r.writes++
	if status.Value != "open" {
		return errors.New("expected task to reopen")
	}
	return nil
}
func (r *rescheduleProgressTaskRepository) SetStatusByUserIDWithPermission(ctx context.Context, userID domain.UserID, taskID domain.TaskID, status domain.TaskStatus, _ int32, _ shared.Capability) error {
	return r.SetStatusByUserID(ctx, userID, taskID, status)
}
func (r *rescheduleProgressTaskRepository) ReadTaskProgressSources(_ context.Context, taskIDs, _ []string, asOf time.Time) (dao.TaskProgressSources, error) {
	r.reads = append(r.reads, asOf)
	index := len(r.reads) - 1
	return dao.TaskProgressSources{
		Counts:   map[string]dao.TaskProgressCounts{taskIDs[0]: r.counts[index]},
		Statuses: map[string]dao.TaskStatus{taskIDs[0]: {Value: "done"}},
	}, nil
}

func TestRescheduleVirtualScheduleReconcilesProgressAtOneAsOf(t *testing.T) {
	now := time.Now().UTC()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	weekday := []string{"sun", "mon", "tue", "wed", "thu", "fri", "sat"}[today.Weekday()]
	start := today.Add(9 * time.Hour)
	root := dao.TaskSchedule{
		ID: "series-1", TaskID: "task-1", Title: "Focus", SeriesID: "series-1", OccurrenceDate: today.Format("2006-01-02"),
		Timezone: "UTC", IntervalWeeks: 1, Frequencies: []dao.TaskFrequency{{Value: weekday}}, RepeatState: repeatStateActive,
		FrequencyAnchorDate: today.Unix(), StartAt: start.Unix(), EndAt: start.Add(time.Hour).Unix(), CreatedAt: now.Unix(),
	}
	schedules := &rescheduleProgressScheduleRepository{root: root}
	progress := &rescheduleProgressTaskRepository{counts: []dao.TaskProgressCounts{{Total: 1, Completed: 1}, {Total: 2, Completed: 1}}}
	repos := rescheduleProgressRepositories{schedules: schedules, tasks: progress}
	uc := NewRescheduleTaskScheduleUseCase(rescheduleProgressUOW{repos: repos}, nil, shared.ID(rescheduleProgressID{}))
	_, err := uc.ExecuteOccurrence(context.Background(), RescheduleTaskScheduleInput{
		UserID: "user-1", TaskID: "task-1", TaskScheduleID: "series-1", OccurrenceDate: today.Format("2006-01-02"),
		StartAt: start.Add(30 * time.Minute), EndAt: start.Add(90 * time.Minute), Scope: taskScheduleScopeCurrent,
	})
	if err != nil {
		t.Fatal(err)
	}
	if schedules.writes != 1 {
		t.Fatalf("override writes=%d, want one saved occurrence", schedules.writes)
	}
	if len(progress.reads) != 2 || !progress.reads[0].Equal(progress.reads[1]) {
		t.Fatalf("progress snapshot times=%v, want two reads with same asOf", progress.reads)
	}
	if progress.writes != 1 {
		t.Fatalf("status writes=%d, want task reopened once after count change", progress.writes)
	}
}
