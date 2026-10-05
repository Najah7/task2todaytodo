package usecase

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/task/dao"
	"github.com/Najah7/task2todaytodo/internal/application/task/domain"
)

type createTaskScheduleUOWFake struct {
	repos      Repositories
	err        error
	calls      int
	committed  bool
	rolledBack bool
}

func (uow *createTaskScheduleUOWFake) Do(ctx context.Context, fn func(context.Context, Repositories) error) error {
	uow.calls++
	if uow.err != nil {
		return uow.err
	}
	if err := fn(ctx, uow.repos); err != nil {
		uow.rolledBack = true
		return err
	}
	uow.committed = true
	return nil
}

type createTaskScheduleRepositoriesFake struct {
	taskProgressTestRepositories
	tasks     TaskRepository
	schedules TaskScheduleRepository
}

func (r createTaskScheduleRepositoriesFake) Tasks() TaskRepository { return r.tasks }
func (r createTaskScheduleRepositoriesFake) TaskSchedules() TaskScheduleRepository {
	return r.schedules
}

type createTaskScheduleTaskRepositoryFake struct{ taskProgressTestRepository }

type createTaskScheduleRepositoryFake struct {
	TaskScheduleRepository
	roots           []domain.TaskSchedule
	occurrences     []domain.TaskSchedule
	rootErr         error
	occurrenceErr   error
	order           []string
	userID          domain.UserID
	occurrenceUsers []domain.UserID
}

func (r *createTaskScheduleRepositoryFake) CreateByTaskAndUserID(_ context.Context, userID domain.UserID, s domain.TaskSchedule) (dao.TaskSchedule, error) {
	r.order = append(r.order, "root")
	r.userID = userID
	r.roots = append(r.roots, s)
	if r.rootErr != nil {
		return dao.TaskSchedule{}, r.rootErr
	}
	return dao.TaskSchedule{ID: string(s.ID), TaskID: string(s.TaskID), Title: s.Title, IntervalWeeks: s.IntervalWeeks, StartAt: s.StartAt.Unix(), EndAt: s.EndAt.Unix(), SeriesID: string(s.SeriesID), OccurrenceDate: s.OccurrenceDate.Format("2006-01-02"), Timezone: s.Timezone}, nil
}
func (r *createTaskScheduleRepositoryFake) CreateOccurrenceByTaskAndUserID(_ context.Context, userID domain.UserID, s domain.TaskSchedule) (dao.TaskSchedule, error) {
	r.order = append(r.order, "occurrence")
	r.occurrenceUsers = append(r.occurrenceUsers, userID)
	r.occurrences = append(r.occurrences, s)
	if r.occurrenceErr != nil {
		return dao.TaskSchedule{}, r.occurrenceErr
	}
	return dao.TaskSchedule{ID: string(s.ID), SeriesID: string(s.SeriesID)}, nil
}

type createTaskScheduleTimezoneFake struct {
	timezone string
	err      error
	calls    int
	userID   domain.UserID
}

func (r *createTaskScheduleTimezoneFake) GetTimezone(_ context.Context, userID domain.UserID) (string, error) {
	r.calls++
	r.userID = userID
	return r.timezone, r.err
}

func createTaskScheduleFixture() (*createTaskScheduleUOWFake, *createTaskScheduleTaskRepositoryFake, *createTaskScheduleRepositoryFake, *createTaskScheduleTimezoneFake, *CreateTaskScheduleUseCase) {
	tasks := &createTaskScheduleTaskRepositoryFake{}
	schedules := &createTaskScheduleRepositoryFake{}
	timezones := &createTaskScheduleTimezoneFake{timezone: "Asia/Tokyo"}
	uow := &createTaskScheduleUOWFake{repos: createTaskScheduleRepositoriesFake{tasks: tasks, schedules: schedules}}
	uc := NewCreateTaskScheduleUseCase(uow, timezones, nil)
	return uow, tasks, schedules, timezones, uc
}

func validCreateTaskScheduleInput() CreateTaskScheduleInput {
	start := time.Date(2026, time.October, 5, 16, 0, 0, 0, time.UTC) // 09:00 in Los Angeles
	return CreateTaskScheduleInput{ID: "schedule-root", UserID: "user-1", TaskID: "task-1", Title: "Weekly planning", Description: "Prepare the week", Location: "Meeting room", StartAt: start, EndAt: start.Add(time.Hour)}
}

func TestCreateTaskScheduleUseCaseCreatesOneOffScheduleAsInitialRoot(t *testing.T) {
	uow, _, schedules, timezones, uc := createTaskScheduleFixture()
	timezones.timezone = "America/Los_Angeles"
	input := validCreateTaskScheduleInput()
	want := dao.TaskSchedule{ID: "schedule-root", TaskID: "task-1", Title: input.Title, IntervalWeeks: 0, StartAt: input.StartAt.Unix(), EndAt: input.EndAt.Unix(), SeriesID: "schedule-root", OccurrenceDate: "2026-10-05", Timezone: "America/Los_Angeles"}

	got, err := uc.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Execute() = %#v, want %#v", got, want)
	}
	if uow.calls != 1 || !uow.committed || uow.rolledBack {
		t.Errorf("UOW = calls:%d committed:%t rolled back:%t, want one commit", uow.calls, uow.committed, uow.rolledBack)
	}
	if timezones.calls != 1 || timezones.userID != input.UserID {
		t.Errorf("timezone lookup = (%d,%q), want one lookup for %q", timezones.calls, timezones.userID, input.UserID)
	}
	if len(schedules.roots) != 1 || len(schedules.occurrences) != 0 {
		t.Fatalf("writes = root:%d occurrences:%d, want root only", len(schedules.roots), len(schedules.occurrences))
	}
	root := schedules.roots[0]
	if root.ID != input.ID || root.SeriesID != input.ID || root.Timezone != timezones.timezone || root.OccurrenceDate.Format("2006-01-02") != "2026-10-05" {
		t.Errorf("root recurrence = (%q,%q,%q,%s), want source as first local occurrence", root.ID, root.SeriesID, root.Timezone, root.OccurrenceDate.Format("2006-01-02"))
	}
}

func TestCreateTaskScheduleUseCaseStoresRecurringRuleWithoutMaterializingOccurrences(t *testing.T) {
	uow, _, schedules, timezones, uc := createTaskScheduleFixture()
	timezones.timezone = "America/Los_Angeles"
	input := validCreateTaskScheduleInput()
	input.IntervalWeeks = domain.WeeklyIntervalWeeks
	input.Frequencies = []string{"mon", "wed"}
	got, err := uc.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if got.ID != string(input.ID) || got.SeriesID != string(input.ID) || got.Timezone != timezones.timezone || got.OccurrenceDate != "2026-10-05" || got.IntervalWeeks != domain.WeeklyIntervalWeeks {
		t.Errorf("created root = %#v, want recurring root in frozen timezone", got)
	}
	if len(schedules.roots) != 1 || len(schedules.occurrences) != 0 {
		t.Fatalf("writes roots=%d occurrences=%d; want root only", len(schedules.roots), len(schedules.occurrences))
	}
	if !uow.committed || uow.rolledBack {
		t.Errorf("UOW commit=%t rollback=%t", uow.committed, uow.rolledBack)
	}
}

func TestCreateTaskScheduleUseCaseKeepsLocalWallTimeOnRecurringRoot(t *testing.T) {
	_, _, schedules, timezones, uc := createTaskScheduleFixture()
	timezones.timezone = "America/New_York"
	input := validCreateTaskScheduleInput()
	input.StartAt = time.Date(2026, time.February, 22, 7, 30, 0, 0, time.UTC)
	input.EndAt = time.Date(2026, time.February, 22, 8, 30, 0, 0, time.UTC)
	input.IntervalWeeks = domain.WeeklyIntervalWeeks
	input.Frequencies = []string{"sun"}
	if _, err := uc.Execute(context.Background(), input); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if len(schedules.roots) != 1 || len(schedules.occurrences) != 0 {
		t.Fatalf("writes root=%d occurrences=%d; want root only", len(schedules.roots), len(schedules.occurrences))
	}
	loc, err := time.LoadLocation(timezones.timezone)
	if err != nil {
		t.Fatal(err)
	}
	local := schedules.roots[0].StartAt.In(loc)
	if local.Hour() != 2 || local.Minute() != 30 {
		t.Errorf("root wall start = %s, want 02:30", local)
	}
}

func TestCreateTaskScheduleUseCaseRejectsInvalidInputBeforeWrites(t *testing.T) {
	tests := []struct {
		name  string
		setup func(*CreateTaskScheduleInput)
		want  error
	}{
		{"end not after start", func(input *CreateTaskScheduleInput) {
			input.EndAt = input.StartAt
		}, domain.ErrTaskScheduleEndAtMustBeAfterStartAt},
		{"invalid frequency", func(input *CreateTaskScheduleInput) {
			input.Frequencies = []string{"bad"}
		}, domain.ErrTaskFrequencyInvalid},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			uow, _, schedules, _, uc := createTaskScheduleFixture()
			input := validCreateTaskScheduleInput()
			test.setup(&input)
			_, err := uc.Execute(context.Background(), input)
			if !errors.Is(err, test.want) || len(schedules.roots) != 0 || len(schedules.occurrences) != 0 || !uow.rolledBack {
				t.Errorf("Execute() error=%v writes=%d/%d rollback=%t, want %v and no writes", err, len(schedules.roots), len(schedules.occurrences), uow.rolledBack, test.want)
			}
		})
	}
}

func TestCreateTaskScheduleUseCaseRollsBackWhenRootInsertFails(t *testing.T) {
	uow, _, schedules, _, uc := createTaskScheduleFixture()
	wantErr := errors.New("root insert failed")
	schedules.rootErr = wantErr
	got, err := uc.Execute(context.Background(), validCreateTaskScheduleInput())
	if !errors.Is(err, wantErr) || !reflect.DeepEqual(got, dao.TaskSchedule{}) {
		t.Errorf("Execute() = (%#v,%v), want root error", got, err)
	}
	if !uow.rolledBack || uow.committed || len(schedules.roots) != 1 || len(schedules.occurrences) != 0 {
		t.Errorf("UOW commit=%t rollback=%t roots=%d occurrences=%d", uow.committed, uow.rolledBack, len(schedules.roots), len(schedules.occurrences))
	}
}

func TestCreateTaskScheduleUseCasePropagatesTimezoneAndRepositoryErrors(t *testing.T) {
	t.Run("timezone reader", func(t *testing.T) {
		uow, _, schedules, reader, uc := createTaskScheduleFixture()
		want := errors.New("timezone lookup failed")
		reader.err = want
		_, err := uc.Execute(context.Background(), validCreateTaskScheduleInput())
		if !errors.Is(err, want) || len(schedules.roots) != 0 || !uow.rolledBack {
			t.Errorf("Execute() error=%v writes=%d rollback=%t, want original error before write", err, len(schedules.roots), uow.rolledBack)
		}
	})
	t.Run("invalid timezone", func(t *testing.T) {
		uow, _, schedules, reader, uc := createTaskScheduleFixture()
		reader.timezone = "not-a-zone"
		_, err := uc.Execute(context.Background(), validCreateTaskScheduleInput())
		if !errors.Is(err, domain.ErrRecurrenceTimezoneInvalid) || len(schedules.roots) != 0 || !uow.rolledBack {
			t.Errorf("Execute() error=%v writes=%d rollback=%t, want invalid timezone and no writes", err, len(schedules.roots), uow.rolledBack)
		}
	})
	t.Run("root repository", func(t *testing.T) {
		uow, _, schedules, _, uc := createTaskScheduleFixture()
		want := errors.New("root insert failed")
		schedules.rootErr = want
		_, err := uc.Execute(context.Background(), validCreateTaskScheduleInput())
		if !errors.Is(err, want) || len(schedules.occurrences) != 0 || !uow.rolledBack {
			t.Errorf("Execute() error=%v occurrences=%d rollback=%t, want original error and no children", err, len(schedules.occurrences), uow.rolledBack)
		}
	})
	t.Run("unit of work", func(t *testing.T) {
		want := errors.New("transaction failed")
		uow := &createTaskScheduleUOWFake{err: want}
		uc := NewCreateTaskScheduleUseCase(uow, &createTaskScheduleTimezoneFake{timezone: "UTC"}, nil)
		_, err := uc.Execute(context.Background(), validCreateTaskScheduleInput())
		if !errors.Is(err, want) || uow.calls != 1 || uow.committed || uow.rolledBack {
			t.Errorf("Execute() error=%v UOW calls=%d, want original UOW error and no callback", err, uow.calls)
		}
	})
}
