package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/task/dao"
	"github.com/Najah7/task2todaytodo/internal/application/task/domain"
	"github.com/oklog/ulid/v2"
)

type sparseReorderIDs struct{}

func (sparseReorderIDs) Generate() string { return ulid.Make().String() }

type reorderTodoItemUOWFake struct {
	repos      Repositories
	calls      int
	committed  bool
	rolledBack bool
	err        error
}

type sparseReorderRepositories struct {
	taskProgressTestRepositories
	items TodoItemRepository
	tasks TaskRepository
}

func (r sparseReorderRepositories) TodoItems() TodoItemRepository { return r.items }
func (r sparseReorderRepositories) Tasks() TaskRepository         { return r.tasks }

type sparseReorderUOW struct{ repos Repositories }

func (u sparseReorderUOW) Do(ctx context.Context, fn func(context.Context, Repositories) error) error {
	return fn(ctx, u.repos)
}

type sparseReorderRepo struct {
	TodoItemRepository
	rows   []dao.TodoItem
	writes int
}

func (r *sparseReorderRepo) ListByTask(context.Context, domain.UserID, domain.TaskID) ([]dao.TodoItem, error) {
	return append([]dao.TodoItem(nil), r.rows...), nil
}
func (r *sparseReorderRepo) UpsertTodoItemOverride(_ context.Context, _ domain.UserID, item domain.TodoItem) (string, error) {
	root := dao.TodoItem{}
	for _, row := range r.rows {
		if row.ID == string(item.SeriesID) {
			root = row
			break
		}
	}
	if root.ID == "" {
		return "", ErrTodoItemNotFound
	}
	id := string(item.ID)
	if id == VirtualOccurrenceID {
		id = "override-" + string(item.SeriesID) + "-" + item.OccurrenceDate.Format("2006-01-02")
	}
	updated := todoDAOFromOccurrence(root, item, id)
	r.writes++
	for i, row := range r.rows {
		if row.SeriesID == string(item.SeriesID) && row.OccurrenceDate == item.OccurrenceDate.Format("2006-01-02") {
			updated.ID = row.ID
			r.rows[i] = updated
			return updated.ID, nil
		}
	}
	r.rows = append(r.rows, updated)
	return updated.ID, nil
}

func newSparseReorderFixture() (*sparseReorderRepo, *ReorderTodoItemUseCase) {
	date := func(y int, m time.Month, d int) int64 { return time.Date(y, m, d, 0, 0, 0, 0, time.UTC).Unix() }
	rows := []dao.TodoItem{
		{ID: "series-a", TaskID: "task-1", Title: "Recurring", IntervalWeeks: 1, Frequencies: []dao.TaskFrequency{{Value: "mon"}}, Position: 0, SeriesID: "series-a", OccurrenceDate: "2026-10-05", Timezone: "UTC"},
		{ID: "series-b", TaskID: "task-1", Title: "B", IntervalWeeks: 0, Position: 1, SeriesID: "series-b", OccurrenceDate: "2026-10-12", Timezone: "UTC", DueDate: date(2026, time.October, 12)},
		{ID: "series-c", TaskID: "task-1", Title: "C", IntervalWeeks: 0, Position: 2, SeriesID: "series-c", OccurrenceDate: "2026-10-12", Timezone: "UTC"},
		{ID: "series-d", TaskID: "task-1", Title: "Other day", IntervalWeeks: 0, Position: 7, SeriesID: "series-d", OccurrenceDate: "2026-10-19", Timezone: "UTC"},
	}
	repo := &sparseReorderRepo{rows: rows}
	uow := sparseReorderUOW{repos: sparseReorderRepositories{items: repo, tasks: &reorderProgressFake{}}}
	return repo, NewReorderTodoItemUseCase(uow, nil, sparseReorderIDs{})
}

func TestReorderTodoItemOccurrenceMovesVirtualAndPersistsOnlyAffectedDate(t *testing.T) {
	repo, uc := newSparseReorderFixture()
	got, err := uc.ExecuteOccurrence(context.Background(), "user-1", "task-1", "series-a", "2026-10-12", 2)
	if err != nil {
		t.Fatalf("ExecuteOccurrence() error=%v", err)
	}
	if got.SeriesID != "series-a" || got.OccurrenceDate != "2026-10-12" || got.Position != 2 || got.ID == VirtualOccurrenceID {
		t.Errorf("target response=%#v", got)
	}
	want := map[string]int{"series-a": 0, "series-b": 0, "series-c": 1, "series-d": 7}
	virtualStored := false
	for _, row := range repo.rows {
		if position, ok := want[row.ID]; ok && row.Position != position {
			t.Errorf("%s position=%d want %d", row.ID, row.Position, position)
		}
		if row.SeriesID == "series-a" && row.OccurrenceDate == "2026-10-12" && row.Position == 2 && !row.Deleted {
			virtualStored = true
		}
	}
	if !virtualStored {
		t.Error("reordered virtual occurrence diff was not persisted at position 2")
	}
	if repo.writes != 3 {
		t.Errorf("sparse override writes=%d want 3 affected occurrences", repo.writes)
	}
}

func TestReorderTodoItemOccurrenceRejectsOutOfRangeWithoutWrites(t *testing.T) {
	repo, uc := newSparseReorderFixture()
	_, err := uc.ExecuteOccurrence(context.Background(), "user-1", "task-1", "series-a", "2026-10-12", 3)
	if !errors.Is(err, ErrTodoItemPositionOutOfRange) || repo.writes != 0 {
		t.Errorf("error=%v writes=%d want range error and no writes", err, repo.writes)
	}
}

type reorderProgressFake struct {
	taskProgressTestRepository
	reads  []time.Time
	writes int
}

func (r *reorderProgressFake) LockByUserID(_ context.Context, userID domain.UserID, taskID domain.TaskID) (dao.Task, error) {
	return dao.Task{ID: string(taskID), UserID: string(userID), Status: dao.TaskStatus{Value: "done"}}, nil
}
func (r *reorderProgressFake) SetStatusByUserID(_ context.Context, _ domain.UserID, _ domain.TaskID, _ domain.TaskStatus) error {
	r.writes++
	return nil
}
func (r *reorderProgressFake) ReadTaskProgressSources(_ context.Context, taskIDs, _ []string, asOf time.Time) (dao.TaskProgressSources, error) {
	r.reads = append(r.reads, asOf)
	return dao.TaskProgressSources{
		Counts:   map[string]dao.TaskProgressCounts{taskIDs[0]: {Total: 1}},
		Statuses: map[string]dao.TaskStatus{taskIDs[0]: {Value: "done"}},
	}, nil
}

func TestReorderTodoItemOccurrenceDoesNotChangeStatusWhenProgressCountsStaySame(t *testing.T) {
	repo, uc := newSparseReorderFixture()
	progress := &reorderProgressFake{}
	uc.uow = sparseReorderUOW{repos: sparseReorderRepositories{items: repo, tasks: progress}}
	if _, err := uc.ExecuteOccurrence(context.Background(), "user-1", "task-1", "series-a", "2026-10-12", 2); err != nil {
		t.Fatal(err)
	}
	if len(progress.reads) != 2 || !progress.reads[0].Equal(progress.reads[1]) {
		t.Fatalf("progress snapshot times=%v, want two reads with same asOf", progress.reads)
	}
	if progress.writes != 0 {
		t.Fatalf("status writes=%d, want none for unchanged counts", progress.writes)
	}
}

func (uow *reorderTodoItemUOWFake) Do(ctx context.Context, fn func(context.Context, Repositories) error) error {
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

type reorderTodoItemRepositoriesFake struct {
	taskProgressTestRepositories
	items TodoItemRepository
}

func (repos *reorderTodoItemRepositoriesFake) TodoItems() TodoItemRepository { return repos.items }

type reorderTodoItemRepositoryFake struct {
	TodoItemRepository
	rows         []dao.TodoItem
	ownerID      domain.UserID
	getErr       error
	listErr      error
	reorderErr   error
	getCalls     int
	listCalls    int
	reorderCalls int
	gotUserID    domain.UserID
	gotTaskID    domain.TaskID
	gotItemID    domain.TodoItemID
	gotPosition  int
}

func (repo *reorderTodoItemRepositoryFake) GetForOwnedTask(_ context.Context, userID domain.UserID, taskID domain.TaskID, id domain.TodoItemID) (dao.TodoItem, error) {
	repo.getCalls++
	repo.gotUserID, repo.gotTaskID, repo.gotItemID = userID, taskID, id
	if repo.getErr != nil {
		return dao.TodoItem{}, repo.getErr
	}
	if userID != repo.ownerID {
		return dao.TodoItem{}, ErrTodoItemNotFound
	}
	for _, row := range repo.rows {
		if row.ID == string(id) && row.TaskID == string(taskID) && !row.Deleted {
			return row, nil
		}
	}
	return dao.TodoItem{}, ErrTodoItemNotFound
}

func (repo *reorderTodoItemRepositoryFake) ListByTask(_ context.Context, userID domain.UserID, taskID domain.TaskID) ([]dao.TodoItem, error) {
	repo.listCalls++
	repo.gotUserID, repo.gotTaskID = userID, taskID
	if repo.listErr != nil {
		return nil, repo.listErr
	}
	if userID != repo.ownerID {
		return nil, ErrTodoItemNotFound
	}
	return append([]dao.TodoItem(nil), repo.rows...), nil
}

func (repo *reorderTodoItemRepositoryFake) ReorderForOwnedTask(_ context.Context, userID domain.UserID, taskID domain.TaskID, id domain.TodoItemID, position int) (dao.TodoItem, error) {
	repo.reorderCalls++
	repo.gotUserID, repo.gotTaskID, repo.gotItemID, repo.gotPosition = userID, taskID, id, position
	if repo.reorderErr != nil {
		return dao.TodoItem{}, repo.reorderErr
	}
	var group []int
	target := -1
	for index, row := range repo.rows {
		if row.TaskID == string(taskID) && row.OccurrenceDate == repo.occurrenceDate(id) && !row.Deleted {
			group = append(group, index)
			if row.ID == string(id) {
				target = len(group) - 1
			}
		}
	}
	if target < 0 || position < 0 || position >= len(group) {
		return dao.TodoItem{}, ErrTodoItemPositionOutOfRange
	}
	ordered := make([]dao.TodoItem, len(group))
	for i, index := range group {
		ordered[i] = repo.rows[index]
	}
	item := ordered[target]
	ordered = append(ordered[:target], ordered[target+1:]...)
	ordered = append(ordered, dao.TodoItem{})
	copy(ordered[position+1:], ordered[position:])
	ordered[position] = item
	for i, index := range group {
		ordered[i].Position = i
		repo.rows[index] = ordered[i]
	}
	for _, row := range repo.rows {
		if row.ID == string(id) {
			return row, nil
		}
	}
	return dao.TodoItem{}, ErrTodoItemNotFound
}

func (repo *reorderTodoItemRepositoryFake) occurrenceDate(id domain.TodoItemID) string {
	for _, row := range repo.rows {
		if row.ID == string(id) {
			return row.OccurrenceDate
		}
	}
	return ""
}

func newReorderTodoItemFixture() (*reorderTodoItemUOWFake, *reorderTodoItemRepositoryFake, *ReorderTodoItemUseCase) {
	rows := []dao.TodoItem{
		{ID: "a", TaskID: "task-1", Position: 0, OccurrenceDate: "2026-10-03"},
		{ID: "b", TaskID: "task-1", Position: 1, OccurrenceDate: "2026-10-03"},
		{ID: "c", TaskID: "task-1", Position: 2, OccurrenceDate: "2026-10-03"},
		{ID: "other-day", TaskID: "task-1", Position: 0, OccurrenceDate: "2026-10-04"},
	}
	repo := &reorderTodoItemRepositoryFake{rows: rows, ownerID: "user-1"}
	repos := &reorderTodoItemRepositoriesFake{items: repo}
	uow := &reorderTodoItemUOWFake{repos: repos}
	return uow, repo, NewReorderTodoItemUseCase(uow, nil)
}

func reorderedIDs(repo *reorderTodoItemRepositoryFake, occurrenceDate string) []string {
	items := make([]dao.TodoItem, 0)
	for _, row := range repo.rows {
		if row.OccurrenceDate == occurrenceDate {
			items = append(items, row)
		}
	}
	for i := 0; i < len(items); i++ {
		for j := i + 1; j < len(items); j++ {
			if items[j].Position < items[i].Position {
				items[i], items[j] = items[j], items[i]
			}
		}
	}
	ids := make([]string, len(items))
	for i, item := range items {
		ids[i] = item.ID
	}
	return ids
}
