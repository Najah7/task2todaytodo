package usecase

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/Najah7/task2todaytodo/internal/application/schedule/dao"
	"github.com/Najah7/task2todaytodo/internal/application/schedule/domain"
	"github.com/Najah7/task2todaytodo/internal/application/shared"
)

type scheduleTagsRepositoryFake struct {
	ScheduleRepository
	schedule      dao.Schedule
	getActor      domain.UserID
	getID         domain.ScheduleID
	getCapability shared.Capability
	listID        domain.ScheduleID
	addActor      domain.UserID
	addID         domain.ScheduleID
	addTagID      string
	removeActor   domain.UserID
	removeID      domain.ScheduleID
	removeTagID   string
	addErr        error
	removeErr     error
	batchActor    domain.UserID
	batchIDs      []domain.ScheduleID
	getErr        error
	batchTags     map[string][]dao.ScheduleTag
}

func (r *scheduleTagsRepositoryFake) GetByUserIDWithPermission(_ context.Context, actor domain.UserID, id domain.ScheduleID, capability shared.Capability) (dao.Schedule, error) {
	r.getActor, r.getID, r.getCapability = actor, id, capability
	return r.schedule, r.getErr
}

func (r *scheduleTagsRepositoryFake) ListTags(_ context.Context, _ domain.UserID, id domain.ScheduleID) ([]dao.ScheduleTag, error) {
	r.listID = id
	return []dao.ScheduleTag{{ID: "tag"}}, nil
}

func (r *scheduleTagsRepositoryFake) AddTag(_ context.Context, actor domain.UserID, id domain.ScheduleID, tagID string) error {
	r.addActor, r.addID, r.addTagID = actor, id, tagID
	return r.addErr
}

func (r *scheduleTagsRepositoryFake) RemoveTag(_ context.Context, actor domain.UserID, id domain.ScheduleID, tagID string) error {
	r.removeActor, r.removeID, r.removeTagID = actor, id, tagID
	return r.removeErr
}

func TestScheduleTagUseCasesPropagateAtomicRepositoryOwnershipErrors(t *testing.T) {
	const actorID, scheduleID, tagID = "editor", "schedule-1", "foreign-tag"
	repository := &scheduleTagsRepositoryFake{schedule: dao.Schedule{ID: scheduleID}}
	uow := scheduleTagsUOWFake{repositories: scheduleTagsRepositoriesFake{schedules: repository}}

	repository.addErr = ErrScheduleTagNotFound
	if err := NewAddTagToScheduleUseCase(uow, nil).Execute(context.Background(), actorID, scheduleID, tagID); !errors.Is(err, ErrScheduleTagNotFound) {
		t.Fatalf("add ownership error = %v, want %v", err, ErrScheduleTagNotFound)
	}
	repository.removeErr = ErrScheduleTagNotFound
	if err := NewRemoveTagFromScheduleUseCase(uow, nil).Execute(context.Background(), actorID, scheduleID, tagID); !errors.Is(err, ErrScheduleTagNotFound) {
		t.Fatalf("remove ownership error = %v, want %v", err, ErrScheduleTagNotFound)
	}
}

func (r *scheduleTagsRepositoryFake) ListTagsForSchedules(_ context.Context, actor domain.UserID, ids []domain.ScheduleID) (map[string][]dao.ScheduleTag, error) {
	r.batchActor, r.batchIDs = actor, append([]domain.ScheduleID(nil), ids...)
	return r.batchTags, nil
}

type scheduleTagsRepositoriesFake struct{ schedules ScheduleRepository }

func (scheduleTagsRepositoriesFake) ProjectLifecycle() shared.ProjectWorkLifecycle { return nil }

func (r scheduleTagsRepositoriesFake) Schedules() ScheduleRepository { return r.schedules }

type scheduleTagsUOWFake struct{ repositories Repositories }

func (u scheduleTagsUOWFake) Do(ctx context.Context, fn func(context.Context, Repositories) error) error {
	return fn(ctx, u.repositories)
}

func TestScheduleTagUseCasesPreserveRequestedOverrideID(t *testing.T) {
	const actorID, rootID, overrideID, tagID = "editor", "series-root", "saved-override", "tag"
	repository := &scheduleTagsRepositoryFake{schedule: dao.Schedule{ID: overrideID, SeriesID: rootID, UserID: "project-owner"}}
	uow := scheduleTagsUOWFake{repositories: scheduleTagsRepositoriesFake{schedules: repository}}

	listed, err := NewListScheduleTagsUseCase(uow, nil).Execute(context.Background(), actorID, overrideID)
	if err != nil || len(listed) != 1 || repository.getID != overrideID || repository.listID != overrideID {
		t.Fatalf("list saved-override tags = %+v, error=%v; permission/list must use requested occurrence ID", listed, err)
	}
	if repository.getCapability != shared.ScheduleRead() {
		t.Fatalf("ListTags permission = %+v; want Schedule read", repository.getCapability)
	}

	if err := NewAddTagToScheduleUseCase(uow, nil).Execute(context.Background(), actorID, overrideID, tagID); err != nil {
		t.Fatal(err)
	}
	if repository.getID != overrideID || repository.getCapability != shared.ScheduleUpdate() || repository.addActor != actorID || repository.addID != overrideID || repository.addTagID != tagID {
		t.Fatalf("add saved-override tag permissions/assignment: repository=%+v", repository)
	}

	if err := NewRemoveTagFromScheduleUseCase(uow, nil).Execute(context.Background(), actorID, overrideID, tagID); err != nil {
		t.Fatal(err)
	}
	if repository.getID != overrideID || repository.getCapability != shared.ScheduleUpdate() || repository.removeActor != actorID || repository.removeID != overrideID || repository.removeTagID != tagID {
		t.Fatalf("remove saved-override tag must preserve requested ID and update permission: %+v", repository)
	}
}

func TestListScheduleTagsForSchedulesBatchesRequestedRows(t *testing.T) {
	const actorID = "editor"
	ids := []domain.ScheduleID{"root", "saved-override", "virtual-root", "saved-override"}
	repository := &scheduleTagsRepositoryFake{batchTags: map[string][]dao.ScheduleTag{
		"root":           {{ID: "tag-a"}},
		"saved-override": {{ID: "tag-a"}, {ID: "tag-b"}},
		"virtual-root":   {},
	}}
	uow := scheduleTagsUOWFake{repositories: scheduleTagsRepositoriesFake{schedules: repository}}
	got, err := NewListScheduleTagsForSchedulesUseCase(uow, nil).Execute(context.Background(), actorID, ids)
	if err != nil {
		t.Fatal(err)
	}
	wantIDs := []domain.ScheduleID{"root", "saved-override", "virtual-root"}
	if repository.batchActor != actorID || !reflect.DeepEqual(repository.batchIDs, wantIDs) {
		t.Fatalf("batch repository call actor=%q IDs=%v; want actor=%q IDs=%v", repository.batchActor, repository.batchIDs, actorID, wantIDs)
	}
	if len(got) != 3 || len(got["root"]) != 1 || len(got["saved-override"]) != 2 || got["virtual-root"] == nil || len(got["virtual-root"]) != 0 {
		t.Fatalf("batch tag map = %#v; want per-request IDs with empty list for untagged row", got)
	}
}
