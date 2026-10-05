package usecase

import (
	"context"
	"testing"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/task/dao"
	"github.com/Najah7/task2todaytodo/internal/application/task/domain"
)

type updateTaskScheduleUOWFake struct{ repos Repositories }

func (u *updateTaskScheduleUOWFake) Do(ctx context.Context, fn func(context.Context, Repositories) error) error {
	return fn(ctx, u.repos)
}

type updateTaskScheduleRepositoriesFake struct {
	taskProgressTestRepositories
	schedules TaskScheduleRepository
}

func (r *updateTaskScheduleRepositoriesFake) TaskSchedules() TaskScheduleRepository {
	return r.schedules
}

type updateTaskScheduleRepositoryFake struct {
	TaskScheduleRepository
	root                 dao.TaskSchedule
	result               dao.TaskSchedule
	updated              domain.TaskSchedule
	templates, overrides int
	snapshotID           domain.TaskScheduleID
}

func (r *updateTaskScheduleRepositoryFake) GetByTaskAndUserID(context.Context, domain.UserID, domain.TaskID, domain.TaskScheduleID) (dao.TaskSchedule, error) {
	return r.root, nil
}
func (r *updateTaskScheduleRepositoryFake) ListByTaskAndUserID(context.Context, domain.UserID, domain.TaskID) ([]dao.TaskSchedule, error) {
	rows := []dao.TaskSchedule{r.root}
	return rows, nil
}
func (r *updateTaskScheduleRepositoryFake) UpsertTaskScheduleOverride(_ context.Context, _ domain.UserID, schedule domain.TaskSchedule) (string, error) {
	r.overrides++
	r.updated = schedule
	return string(schedule.ID), nil
}
func (r *updateTaskScheduleRepositoryFake) UpdateTaskScheduleSeriesTemplate(_ context.Context, _ domain.UserID, _ domain.TaskID, _ domain.TaskScheduleID, snapshotID domain.TaskScheduleID, schedule domain.TaskSchedule) (dao.TaskSchedule, error) {
	r.templates++
	r.snapshotID = snapshotID
	r.updated = schedule
	r.root.Title = schedule.Title
	return dao.TaskSchedule{ID: r.root.ID, TaskID: r.root.TaskID, SeriesID: r.root.ID, OccurrenceDate: r.root.OccurrenceDate, Title: schedule.Title, IntervalWeeks: r.root.IntervalWeeks, RepeatState: r.root.RepeatState, FrequencyAnchorDate: r.root.FrequencyAnchorDate, Timezone: r.root.Timezone, StartAt: r.root.StartAt, EndAt: r.root.EndAt}, nil
}
func (r *updateTaskScheduleRepositoryFake) UpdateByTaskAndUserID(_ context.Context, _ domain.UserID, schedule domain.TaskSchedule) (dao.TaskSchedule, error) {
	r.updated = schedule
	return dao.TaskSchedule{ID: string(schedule.ID), TaskID: string(schedule.TaskID), SeriesID: string(schedule.SeriesID), OccurrenceDate: schedule.OccurrenceDate.Format("2006-01-02"), Title: schedule.Title, Timezone: schedule.Timezone}, nil
}

func updateTaskScheduleRow(id string) dao.TaskSchedule {
	start := time.Date(2026, 10, 5, 9, 0, 0, 0, time.UTC)
	return dao.TaskSchedule{ID: id, TaskID: "task-1", Title: "Weekly planning", Description: "Original description", Location: "Room A", IntervalWeeks: 1, Frequencies: []dao.TaskFrequency{{Value: "mon"}}, RepeatState: repeatStateActive, FrequencyAnchorDate: mustParseDate("2026-10-05").Unix(), StartAt: start.Unix(), EndAt: start.Add(time.Hour).Unix(), SeriesID: id, OccurrenceDate: "2026-10-05", Timezone: "UTC", CreatedAt: 100, UpdatedAt: 200}
}

func TestUpdateTaskScheduleFutureScopeSnapshotsRootAndKeepsAnchor(t *testing.T) {
	root := updateTaskScheduleRow("schedule-root")
	repo := &updateTaskScheduleRepositoryFake{root: root}
	uow := &updateTaskScheduleUOWFake{repos: &updateTaskScheduleRepositoriesFake{schedules: repo}}
	uc := NewUpdateTaskScheduleUseCase(uow, nil, fixedUpdateID("snapshot-id"))
	title := "future title"
	got, err := uc.ExecuteOccurrence(context.Background(), "user-1", "task-1", "schedule-root", "2026-10-05", taskScheduleScopeFuture, PatchField[string]{Present: true, Value: &title}, PatchField[string]{}, PatchField[string]{})
	if err != nil {
		t.Fatal(err)
	}
	if repo.templates != 1 || repo.snapshotID != "snapshot-id" || repo.updated.Title != title || repo.root.FrequencyAnchorDate != root.FrequencyAnchorDate || got.Title != title {
		t.Fatalf("template calls=%d snapshot=%s updated=%#v result=%#v", repo.templates, repo.snapshotID, repo.updated, got)
	}
}

func TestUpdateTaskScheduleCurrentOccurrenceStoresOverride(t *testing.T) {
	root := updateTaskScheduleRow("schedule-root")
	repo := &updateTaskScheduleRepositoryFake{root: root}
	uow := &updateTaskScheduleUOWFake{repos: &updateTaskScheduleRepositoriesFake{schedules: repo}}
	uc := NewUpdateTaskScheduleUseCase(uow, nil, fixedUpdateID("saved-id"))
	title := "renamed"
	got, err := uc.ExecuteOccurrence(context.Background(), "user-1", "task-1", "schedule-root", "2026-10-05", taskScheduleScopeCurrent, PatchField[string]{Present: true, Value: &title}, PatchField[string]{}, PatchField[string]{})
	if err != nil {
		t.Fatal(err)
	}
	if repo.overrides != 1 || !repo.updated.IsException || got.Title != title {
		t.Fatalf("overrides=%d updated=%#v result=%#v", repo.overrides, repo.updated, got)
	}
}
