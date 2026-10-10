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

type createActionItemUOWFake struct {
	repos      Repositories
	err        error
	calls      int
	committed  bool
	rolledBack bool
}

func (uow *createActionItemUOWFake) Do(ctx context.Context, fn func(context.Context, Repositories) error) error {
	uow.calls++
	if uow.err != nil {
		return uow.err
	}
	err := fn(ctx, uow.repos)
	if err != nil {
		uow.rolledBack = true
		return err
	}
	uow.committed = true
	return nil
}

type createActionItemRepositoriesFake struct {
	taskProgressTestRepositories
	tasks       TaskRepository
	actionItems ActionItemRepository
}

func (repos createActionItemRepositoriesFake) Tasks() TaskRepository { return repos.tasks }
func (repos createActionItemRepositoriesFake) ActionItems() ActionItemRepository {
	return repos.actionItems
}

type createActionItemTaskRepositoryFake struct{ taskProgressTestRepository }

type createActionItemRepositoryFake struct {
	ActionItemRepository
	created       []domain.ActionItem
	createErr     error
	occurrenceErr error
	appendTail    []bool
}

func (*createActionItemRepositoryFake) ReadTaskListProjection(_ context.Context, _ domain.UserID, taskIDs []string) (dao.TaskListProjectionSources, error) {
	items := make(map[string][]dao.ActionItem, len(taskIDs))
	skipped := make(map[string]map[string]map[string]bool, len(taskIDs))
	for _, id := range taskIDs {
		items[id] = nil
		skipped[id] = nil
	}
	return dao.TaskListProjectionSources{ActionItemsByTask: items, SkippedByTask: skipped}, nil
}

func (repo *createActionItemRepositoryFake) CreateForOwnedTask(_ context.Context, _ domain.UserID, item domain.ActionItem, appendToTail bool) (dao.ActionItem, error) {
	repo.created = append(repo.created, item)
	repo.appendTail = append(repo.appendTail, appendToTail)
	if repo.createErr != nil {
		return dao.ActionItem{}, repo.createErr
	}
	var frequencies []dao.TaskFrequency
	if len(item.Frequencies) > 0 {
		frequencies = make([]dao.TaskFrequency, 0, len(item.Frequencies))
	}
	for _, frequency := range item.Frequencies {
		frequencies = append(frequencies, dao.TaskFrequency{Value: frequency.String()})
	}
	return dao.ActionItem{ID: string(item.ID), TaskID: string(item.TaskID), Title: item.Title, Position: 4, IntervalWeeks: item.IntervalWeeks, Frequencies: frequencies, SeriesID: string(item.SeriesID), OccurrenceDate: item.OccurrenceDate.Format("2006-01-02"), Timezone: item.Timezone}, nil
}

func (repo *createActionItemRepositoryFake) CreateOccurrenceForOwnedTask(_ context.Context, _ domain.UserID, item domain.ActionItem) (dao.ActionItem, error) {
	repo.created = append(repo.created, item)
	if repo.occurrenceErr != nil {
		return dao.ActionItem{}, repo.occurrenceErr
	}
	return dao.ActionItem{ID: string(item.ID), SeriesID: string(item.SeriesID), OccurrenceDate: item.OccurrenceDate.Format("2006-01-02")}, nil
}

type createActionItemTimezoneFake struct {
	timezone string
	err      error
	calls    int
	userID   domain.UserID
}

func (reader *createActionItemTimezoneFake) GetTimezone(_ context.Context, userID string) (string, error) {
	reader.calls++
	reader.userID = domain.UserID(userID)
	return reader.timezone, reader.err
}

func createActionItemFixture() (*createActionItemUOWFake, *createActionItemTaskRepositoryFake, *createActionItemRepositoryFake, *createActionItemTimezoneFake, *CreateActionItemUseCase) {
	taskRepo := &createActionItemTaskRepositoryFake{}
	itemRepo := &createActionItemRepositoryFake{}
	timezoneReader := &createActionItemTimezoneFake{timezone: "Asia/Tokyo"}
	uow := &createActionItemUOWFake{repos: createActionItemRepositoriesFake{tasks: taskRepo, actionItems: itemRepo}}
	uc := NewCreateActionItemUseCase(uow, timezoneReader, nil)
	uc.clock = func() time.Time { return time.Date(2026, time.October, 5, 1, 0, 0, 0, time.UTC) }
	return uow, taskRepo, itemRepo, timezoneReader, uc
}

func validCreateActionItemInput() CreateActionItemInput {
	return CreateActionItemInput{
		ID:          domain.ActionItemID("item-1"),
		UserID:      domain.UserID("user-1"),
		TaskID:      domain.TaskID("task-1"),
		Title:       "Prepare release",
		Description: "Check changelog",
	}
}

func TestCreateActionItemUseCaseCreatesOneOffItemAtTaskTail(t *testing.T) {
	uow, _, itemRepo, timezoneReader, uc := createActionItemFixture()
	input := validCreateActionItemInput()
	want := dao.ActionItem{ID: "item-1", TaskID: "task-1", Title: input.Title, Position: 4, SeriesID: "item-1", OccurrenceDate: "2026-10-05", Timezone: "Asia/Tokyo"}

	got, err := uc.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Execute() = %#v, want %#v", got, want)
	}
	if uow.calls != 1 || !uow.committed || uow.rolledBack {
		t.Errorf("UOW state = calls %d, committed %t, rolled back %t; want one commit", uow.calls, uow.committed, uow.rolledBack)
	}
	if timezoneReader.calls != 1 || timezoneReader.userID != input.UserID {
		t.Errorf("timezone lookup = (%d, %q), want one lookup for %q", timezoneReader.calls, timezoneReader.userID, input.UserID)
	}
	if len(itemRepo.created) != 1 || len(itemRepo.appendTail) != 1 || !itemRepo.appendTail[0] {
		t.Fatalf("created items = %d, append flags = %v; want source only appended to tail", len(itemRepo.created), itemRepo.appendTail)
	}
	root := itemRepo.created[0]
	if root.ID != input.ID || root.SeriesID != input.ID || root.OccurrenceDate.Format("2006-01-02") != "2026-10-05" || root.Timezone != "Asia/Tokyo" || root.IntervalWeeks != domain.OnceIntervalWeeks {
		t.Errorf("stored source recurrence = (%q,%q,%s,%q,%d), want source-root metadata", root.ID, root.SeriesID, root.OccurrenceDate.Format("2006-01-02"), root.Timezone, root.IntervalWeeks)
	}
}

func TestCreateActionItemUseCaseStoresRecurringRuleWithoutMaterializingOccurrences(t *testing.T) {
	uow, _, itemRepo, timezoneReader, uc := createActionItemFixture()
	timezoneReader.timezone = "America/Los_Angeles"
	input := validCreateActionItemInput()
	input.DueDate = time.Date(2026, time.October, 5, 0, 0, 0, 0, time.UTC)
	input.IntervalWeeks = domain.WeeklyIntervalWeeks
	input.Frequencies = []string{"mon", "wed"}
	got, err := uc.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if got.ID != string(input.ID) || got.SeriesID != string(input.ID) || got.IntervalWeeks != domain.WeeklyIntervalWeeks || len(got.Frequencies) != 2 {
		t.Errorf("root response = %#v, want root series metadata and initial frequencies", got)
	}
	if len(itemRepo.created) != 1 {
		t.Fatalf("physical rows=%d; want root only", len(itemRepo.created))
	}
	root := itemRepo.created[0]
	if root.ID != input.ID || root.SeriesID != input.ID || root.Timezone != timezoneReader.timezone || root.OccurrenceDate.Format("2006-01-02") != "2026-10-05" || root.IntervalWeeks != domain.WeeklyIntervalWeeks || len(root.Frequencies) != 2 || root.Frequencies[0].String() != "mon" || root.Frequencies[1].String() != "wed" {
		t.Errorf("root rule = %#v, want root with full recurrence rule", root)
	}
	if !uow.committed || uow.rolledBack {
		t.Errorf("UOW commit=%t rollback=%t", uow.committed, uow.rolledBack)
	}
}

func TestCreateActionItemUseCaseLeavesDueDateEmptyForUndatedRecurrence(t *testing.T) {
	_, _, itemRepo, _, uc := createActionItemFixture()
	input := validCreateActionItemInput()
	input.IntervalWeeks = domain.WeeklyIntervalWeeks
	input.Frequencies = []string{"mon"}

	_, err := uc.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}
	if len(itemRepo.created) != 1 {
		t.Fatalf("created rows = %d, want only root", len(itemRepo.created))
	}
	for index, item := range itemRepo.created {
		if !item.DueDate.IsZero() {
			t.Errorf("item %d due date = %v, want zero for an undated series", index, item.DueDate)
		}
		if item.OccurrenceDate.IsZero() {
			t.Errorf("item %d occurrence date is zero, want root local date", index)
		}
	}
}

func TestCreateActionItemUseCaseRejectsInvalidInputBeforeCreate(t *testing.T) {
	t.Run("invalid frequency", func(t *testing.T) {
		uow, _, itemRepo, _, uc := createActionItemFixture()
		input := validCreateActionItemInput()
		input.Frequencies = []string{"not-a-weekday"}
		_, err := uc.Execute(context.Background(), input)
		if !errors.Is(err, domain.ErrTaskFrequencyInvalid) || len(itemRepo.created) != 0 || !uow.rolledBack {
			t.Errorf("Execute() error = %v, writes %d, rolled back %t; want invalid frequency and no writes", err, len(itemRepo.created), uow.rolledBack)
		}
	})
}

func TestCreateActionItemUseCaseRollsBackWhenRootInsertFails(t *testing.T) {
	uow, _, itemRepo, _, uc := createActionItemFixture()
	wantErr := errors.New("root insert failed")
	itemRepo.createErr = wantErr
	got, err := uc.Execute(context.Background(), validCreateActionItemInput())
	if !errors.Is(err, wantErr) || !reflect.DeepEqual(got, dao.ActionItem{}) {
		t.Errorf("Execute() = (%#v,%v), want root error", got, err)
	}
	if !uow.rolledBack || uow.committed || len(itemRepo.created) != 1 {
		t.Errorf("UOW committed=%t rollback=%t attempted roots=%d", uow.committed, uow.rolledBack, len(itemRepo.created))
	}
}

func TestCreateActionItemUseCasePropagatesTimezoneAndUOWErrors(t *testing.T) {
	t.Run("timezone reader", func(t *testing.T) {
		uow, _, itemRepo, timezoneReader, uc := createActionItemFixture()
		wantErr := errors.New("user unavailable")
		timezoneReader.err = wantErr
		_, err := uc.Execute(context.Background(), validCreateActionItemInput())
		if !errors.Is(err, wantErr) || len(itemRepo.created) != 0 || !uow.rolledBack {
			t.Errorf("Execute() error = %v, writes %d, rolled back %t; want original error and no writes", err, len(itemRepo.created), uow.rolledBack)
		}
	})
	t.Run("unit of work", func(t *testing.T) {
		uow, _, _, _, uc := createActionItemFixture()
		wantErr := errors.New("transaction unavailable")
		uow.err = wantErr
		got, err := uc.Execute(context.Background(), validCreateActionItemInput())
		if !errors.Is(err, wantErr) || !reflect.DeepEqual(got, dao.ActionItem{}) || uow.calls != 1 {
			t.Errorf("Execute() = (%#v, %v), UOW calls %d; want zero result and original error", got, err, uow.calls)
		}
	})
}
