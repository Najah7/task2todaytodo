package usecase

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/schedule/dao"
	"github.com/Najah7/task2todaytodo/internal/application/schedule/domain"
	"github.com/Najah7/task2todaytodo/internal/application/shared"
)

type scheduleProjectLifecycleFake struct {
	events        *[]string
	states        map[string]shared.ProjectWorkState
	lockErr       error
	lockIDs       []string
	captureIDs    []string
	reconcileIDs  []string
	captureTimes  []time.Time
	reconcileAsOf []time.Time
	actors        []string
	reconciled    []shared.ProjectWorkState
}

func (f *scheduleProjectLifecycleFake) LockParent(_ context.Context, projectID string) error {
	*f.events = append(*f.events, "project.lock:"+projectID)
	f.lockIDs = append(f.lockIDs, projectID)
	return f.lockErr
}

func (f *scheduleProjectLifecycleFake) ReadStatus(context.Context, string) (string, error) {
	return "open", nil
}

func (f *scheduleProjectLifecycleFake) CaptureWorkState(_ context.Context, projectID string, asOf time.Time) (shared.ProjectWorkState, error) {
	*f.events = append(*f.events, "project.capture:"+projectID)
	f.captureIDs = append(f.captureIDs, projectID)
	f.captureTimes = append(f.captureTimes, asOf)
	return f.states[projectID], nil
}

func (f *scheduleProjectLifecycleFake) ReconcileWorkState(_ context.Context, actorID, projectID string, before shared.ProjectWorkState, asOf time.Time) error {
	*f.events = append(*f.events, "project.reconcile:"+projectID)
	f.reconcileIDs = append(f.reconcileIDs, projectID)
	f.actors = append(f.actors, actorID)
	f.reconciled = append(f.reconciled, before)
	f.reconcileAsOf = append(f.reconcileAsOf, asOf)
	return nil
}

type scheduleProjectMutationRepositoryFake struct {
	ScheduleRepository
	events       *[]string
	rows         []dao.Schedule
	reads        int
	setProjectID *domain.ProjectID
	permissions  map[shared.Capability]bool
	createCalled bool
}

func (f *scheduleProjectMutationRepositoryFake) GetByUserIDWithPermission(context.Context, domain.UserID, domain.ScheduleID, shared.Capability) (dao.Schedule, error) {
	*f.events = append(*f.events, "schedule.get")
	index := f.reads
	f.reads++
	if index >= len(f.rows) {
		index = len(f.rows) - 1
	}
	return f.rows[index], nil
}

func (f *scheduleProjectMutationRepositoryFake) LockSeriesProjectForMutation(_ context.Context, _ domain.UserID, seriesID domain.ScheduleID, _ shared.Capability) error {
	*f.events = append(*f.events, "schedule.lock-series:"+string(seriesID))
	return nil
}

func (f *scheduleProjectMutationRepositoryFake) SetProjectByUserID(_ context.Context, _ domain.UserID, _ domain.ScheduleID, projectID *domain.ProjectID) error {
	*f.events = append(*f.events, "schedule.set-project")
	f.setProjectID = projectID
	return nil
}

func (f *scheduleProjectMutationRepositoryFake) CheckProjectPermission(_ context.Context, _ domain.UserID, projectID domain.ProjectID, capability shared.Capability) (bool, error) {
	*f.events = append(*f.events, "schedule.permission:"+string(projectID)+":"+string(capability.Action))
	if f.permissions == nil {
		return true, nil
	}
	allowed, ok := f.permissions[capability]
	return !ok || allowed, nil
}

func (f *scheduleProjectMutationRepositoryFake) LockProjectForScheduleMutation(_ context.Context, projectID domain.ProjectID) error {
	*f.events = append(*f.events, "schedule.lock-project:"+string(projectID))
	return nil
}

func (f *scheduleProjectMutationRepositoryFake) CreateByUserID(_ context.Context, _ domain.UserID, schedule domain.Schedule) (dao.Schedule, error) {
	*f.events = append(*f.events, "schedule.create")
	f.createCalled = true
	return dao.Schedule{ID: string(schedule.ID)}, nil
}

type scheduleProjectMutationRepositoriesFake struct {
	schedules ScheduleRepository
	lifecycle shared.ProjectWorkLifecycle
}

type scheduleProjectMutationUOWFake struct{ repos Repositories }

func (f scheduleProjectMutationUOWFake) Do(ctx context.Context, fn func(context.Context, Repositories) error) error {
	return fn(ctx, f.repos)
}

type scheduleTimezoneReaderFake string

func (f scheduleTimezoneReaderFake) GetTimezone(context.Context, string) (string, error) {
	return string(f), nil
}

func (f scheduleProjectMutationRepositoriesFake) Schedules() ScheduleRepository { return f.schedules }
func (f scheduleProjectMutationRepositoriesFake) ProjectLifecycle() shared.ProjectWorkLifecycle {
	return f.lifecycle
}

func TestWithScheduleProjectMutationLocksParentBeforeScheduleAndReconcilesAfter(t *testing.T) {
	ctx := context.Background()
	asOf := time.Date(2026, 10, 9, 3, 0, 0, 0, time.UTC)
	state := shared.ProjectWorkState{Status: "done", Eligible: 3, Completed: 3}
	events := []string{}
	lifecycle := &scheduleProjectLifecycleFake{events: &events, states: map[string]shared.ProjectWorkState{"project-1": state}}
	row := dao.Schedule{ID: "schedule-1", SeriesID: "series-1", ProjectID: "project-1"}
	repository := &scheduleProjectMutationRepositoryFake{events: &events, rows: []dao.Schedule{row}}
	repos := scheduleProjectMutationRepositoriesFake{schedules: repository, lifecycle: lifecycle}

	err := withScheduleProjectMutation(ctx, repos, "editor", "schedule-1", asOf, shared.ScheduleUpdate(), func(got dao.Schedule, before scheduleProjectMutationSnapshots) error {
		if got.ID != row.ID || before[row.ProjectID] != state {
			t.Fatalf("mutation input schedule=%+v before=%v; want schedule=%+v state=%+v", got, before, row, state)
		}
		events = append(events, "schedule.mutate")
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"schedule.get", "project.lock:project-1", "project.capture:project-1",
		"schedule.get", "schedule.lock-series:series-1", "schedule.get",
		"schedule.mutate", "project.reconcile:project-1",
	}
	if !reflect.DeepEqual(events, want) {
		t.Fatalf("lifecycle call order = %v; want %v", events, want)
	}
	if !reflect.DeepEqual(lifecycle.captureTimes, []time.Time{asOf}) || !reflect.DeepEqual(lifecycle.reconcileAsOf, []time.Time{asOf}) {
		t.Fatalf("snapshot/reconcile times = %v/%v; want same fixed time %v", lifecycle.captureTimes, lifecycle.reconcileAsOf, asOf)
	}
	if !reflect.DeepEqual(lifecycle.actors, []string{"editor"}) || !reflect.DeepEqual(lifecycle.reconciled, []shared.ProjectWorkState{state}) {
		t.Fatalf("reconcile arguments actor=%v before=%v; want editor and captured state %+v", lifecycle.actors, lifecycle.reconciled, state)
	}
}

func TestScheduleProjectMutationDetectsAssociationChangeAfterParentLock(t *testing.T) {
	ctx := context.Background()
	events := []string{}
	lifecycle := &scheduleProjectLifecycleFake{events: &events, states: map[string]shared.ProjectWorkState{"old-project": {Status: "open"}}}
	repository := &scheduleProjectMutationRepositoryFake{events: &events, rows: []dao.Schedule{
		{ID: "schedule-1", SeriesID: "series-1", ProjectID: "old-project"},
		{ID: "schedule-1", SeriesID: "series-1", ProjectID: "new-project"},
	}}
	repos := scheduleProjectMutationRepositoriesFake{schedules: repository, lifecycle: lifecycle}
	mutated := false
	err := withScheduleProjectMutation(ctx, repos, "editor", "schedule-1", time.Now(), shared.ScheduleUpdate(), func(dao.Schedule, scheduleProjectMutationSnapshots) error {
		mutated = true
		return nil
	})
	if err != ErrScheduleProjectChanged || mutated {
		t.Fatalf("association change error=%v mutated=%t; want conflict before child mutation", err, mutated)
	}
	want := []string{"schedule.get", "project.lock:old-project", "project.capture:old-project", "schedule.get"}
	if !reflect.DeepEqual(events, want) {
		t.Fatalf("call order after stale relation = %v; want %v", events, want)
	}
}

func TestScheduleProjectMoveLocksUniqueParentsInIDOrderBeforeCapturing(t *testing.T) {
	ctx := context.Background()
	events := []string{}
	lifecycle := &scheduleProjectLifecycleFake{
		events: &events,
		states: map[string]shared.ProjectWorkState{"project-a": {Status: "open"}, "project-b": {Status: "done"}},
	}
	before, err := lockAndCaptureScheduleProjects(ctx, scheduleProjectMutationRepositoriesFake{lifecycle: lifecycle}, []string{"project-b", "", "project-a", "project-b"}, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"project.lock:project-a", "project.lock:project-b",
		"project.capture:project-a", "project.capture:project-b",
	}
	if !reflect.DeepEqual(events, want) {
		t.Fatalf("multi-Project lock/capture order = %v; want %v", events, want)
	}
	if !reflect.DeepEqual(sortedSnapshotProjectIDs(before), []string{"project-a", "project-b"}) {
		t.Fatalf("captured Project IDs = %v", sortedSnapshotProjectIDs(before))
	}
}

func TestScheduleProjectMoveLocksBothParentsThenReconcilesBoth(t *testing.T) {
	ctx := context.Background()
	asOf := time.Date(2026, 10, 9, 3, 0, 0, 0, time.UTC)
	events := []string{}
	lifecycle := &scheduleProjectLifecycleFake{
		events: &events,
		states: map[string]shared.ProjectWorkState{"project-a": {Status: "open"}, "project-b": {Status: "done"}},
	}
	repository := &scheduleProjectMutationRepositoryFake{
		events: &events,
		rows:   []dao.Schedule{{ID: "schedule-1", SeriesID: "series-1", ProjectID: "project-b"}},
	}
	target := domain.ProjectID("project-a")
	repos := scheduleProjectMutationRepositoriesFake{schedules: repository, lifecycle: lifecycle}
	if err := withScheduleProjectMove(ctx, repos, "editor", "schedule-1", &target, nil, asOf); err != nil {
		t.Fatal(err)
	}
	want := []string{
		"schedule.get", "schedule.permission:project-b:update", "schedule.permission:project-a:create",
		"project.lock:project-a", "project.lock:project-b",
		"project.capture:project-a", "project.capture:project-b",
		"schedule.get", "schedule.lock-series:series-1", "schedule.get",
		"schedule.set-project", "project.reconcile:project-a", "project.reconcile:project-b",
	}
	if !reflect.DeepEqual(events, want) {
		t.Fatalf("Project move call order = %v; want %v", events, want)
	}
	if repository.setProjectID == nil || *repository.setProjectID != target {
		t.Fatalf("assigned Project = %v; want %q", repository.setProjectID, target)
	}
	if !reflect.DeepEqual(lifecycle.reconcileIDs, []string{"project-a", "project-b"}) || !reflect.DeepEqual(lifecycle.reconcileAsOf, []time.Time{asOf, asOf}) {
		t.Fatalf("reconciled IDs/times = %v/%v; want both parents at %v", lifecycle.reconcileIDs, lifecycle.reconcileAsOf, asOf)
	}
}

func TestScheduleProjectMoveChecksTargetPermissionBeforeProjectLifecycle(t *testing.T) {
	events := []string{}
	lifecycle := &scheduleProjectLifecycleFake{events: &events}
	repository := &scheduleProjectMutationRepositoryFake{
		events: &events,
		rows:   []dao.Schedule{{ID: "schedule-1", SeriesID: "series-1", ProjectID: "source"}},
		permissions: map[shared.Capability]bool{
			shared.ScheduleUpdate(): true,
			shared.ScheduleCreate(): false,
		},
	}
	target := domain.ProjectID("target")
	err := withScheduleProjectMove(context.Background(), scheduleProjectMutationRepositoriesFake{
		schedules: repository,
		lifecycle: lifecycle,
	}, "editor", "schedule-1", &target, nil, time.Now())
	if err != ErrPermissionDenied {
		t.Fatalf("move error = %v; want %v", err, ErrPermissionDenied)
	}
	want := []string{
		"schedule.get", "schedule.permission:source:update", "schedule.permission:target:create",
	}
	if !reflect.DeepEqual(events, want) {
		t.Fatalf("events for unauthorized target = %v; want %v", events, want)
	}
}

func TestCreateScheduleChecksProjectPermissionBeforeLifecycleLock(t *testing.T) {
	ctx := context.Background()
	events := []string{}
	lifecycle := &scheduleProjectLifecycleFake{events: &events}
	repository := &scheduleProjectMutationRepositoryFake{
		events: &events,
		permissions: map[shared.Capability]bool{
			shared.ScheduleCreate(): false,
		},
	}
	useCase := NewCreateScheduleUseCase(scheduleProjectMutationUOWFake{
		repos: scheduleProjectMutationRepositoriesFake{schedules: repository, lifecycle: lifecycle},
	}, scheduleTimezoneReaderFake("UTC"), nil)
	start := time.Date(2026, 10, 10, 9, 0, 0, 0, time.UTC)
	_, err := useCase.Execute(ctx, CreateScheduleInput{
		ID: "schedule-1", UserID: "editor", ProjectID: "target-project", Title: "Review",
		StartAt: start, EndAt: start.Add(time.Hour),
	})
	if err != ErrPermissionDenied {
		t.Fatalf("create error = %v; want %v", err, ErrPermissionDenied)
	}
	if repository.createCalled {
		t.Fatal("CreateByUserID ran without project schedule:create permission")
	}
	want := []string{"schedule.permission:target-project:create"}
	if !reflect.DeepEqual(events, want) {
		t.Fatalf("events for unauthorized create = %v; want %v", events, want)
	}
}

func TestLinkedScheduleMutationRequiresProjectLifecyclePort(t *testing.T) {
	row := dao.Schedule{ID: "schedule-1", SeriesID: "series-1", ProjectID: "project-1"}
	events := []string{}
	repository := &scheduleProjectMutationRepositoryFake{events: &events, rows: []dao.Schedule{row}}
	repos := scheduleProjectMutationRepositoriesFake{schedules: repository}
	err := withScheduleProjectMutation(context.Background(), repos, "editor", "schedule-1", time.Now(), shared.ScheduleUpdate(), func(dao.Schedule, scheduleProjectMutationSnapshots) error {
		t.Fatal("mutation ran without Project lifecycle port")
		return nil
	})
	if err != ErrProjectLifecycleUnavailable {
		t.Fatalf("missing lifecycle port error = %v; want %v", err, ErrProjectLifecycleUnavailable)
	}
}

func TestProjectUnavailableDuringLifecycleLockMapsToScheduleNotFound(t *testing.T) {
	events := []string{}
	lifecycle := &scheduleProjectLifecycleFake{
		events:  &events,
		lockErr: shared.ErrProjectUnavailable,
	}
	got, err := lockAndCaptureScheduleProjects(context.Background(), scheduleProjectMutationRepositoriesFake{lifecycle: lifecycle}, []string{"trashed-project"}, time.Now())
	if err != ErrScheduleProjectNotFound || got != nil {
		t.Fatalf("lock/capture result = %v, %v; want nil and %v", got, err, ErrScheduleProjectNotFound)
	}
	if !reflect.DeepEqual(events, []string{"project.lock:trashed-project"}) {
		t.Fatalf("lifecycle calls = %v; want only parent lock", events)
	}
}

func TestDoneProjectSuppressesOnlyUnsavedVirtualScheduleMutations(t *testing.T) {
	done := scheduleProjectMutationSnapshots{"project-1": {Status: "done"}}
	open := scheduleProjectMutationSnapshots{"project-1": {Status: "open"}}
	for _, test := range []struct {
		name           string
		before         scheduleProjectMutationSnapshots
		state          occurrenceState
		restoringSkip  bool
		wantSuppressed bool
	}{
		{name: "done parent suppresses virtual", before: done, state: occurrenceState{}, wantSuppressed: true},
		{name: "done parent permits saved override", before: done, state: occurrenceState{current: &dao.Schedule{ID: "saved"}}},
		{name: "done parent permits root occurrence", before: done, state: occurrenceState{rootOccurrence: true}},
		{name: "done parent permits restoring skipped virtual", before: done, state: occurrenceState{skipped: true}, restoringSkip: true},
		{name: "done parent suppresses skipping virtual", before: done, state: occurrenceState{skipped: true}, wantSuppressed: true},
		{name: "open parent permits virtual", before: open, state: occurrenceState{}},
		{name: "unlinked schedule permits virtual", before: nil, state: occurrenceState{}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := scheduleVirtualOccurrenceSuppressed(test.before, test.state, test.restoringSkip); got != test.wantSuppressed {
				t.Fatalf("suppressed = %t; want %t", got, test.wantSuppressed)
			}
		})
	}
}
