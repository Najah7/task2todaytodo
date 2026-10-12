package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/schedule/dao"
	"github.com/Najah7/task2todaytodo/internal/application/shared"
)

func TestOccurrenceStateTransitionsProtectInactiveAndCompletedRows(t *testing.T) {
	tests := []struct {
		name    string
		current dao.ScheduleOccurrence
		state   string
		wantErr error
	}{
		{name: "skip completed", current: dao.ScheduleOccurrence{Completed: true}, state: "skip", wantErr: ErrOccurrenceCompleted},
		{name: "complete skipped", current: dao.ScheduleOccurrence{Skipped: true}, state: "complete", wantErr: ErrOccurrenceInactive},
		{name: "delete skipped", current: dao.ScheduleOccurrence{Skipped: true}, state: "delete", wantErr: ErrOccurrenceInactive},
		{name: "restore deleted tail", current: dao.ScheduleOccurrence{Deleted: true, Skipped: true}, state: "restore", wantErr: ErrOccurrenceInactive},
		{name: "restore active row", current: dao.ScheduleOccurrence{}, state: "restore", wantErr: ErrOccurrenceNotFound},
		{name: "restore skipped row", current: dao.ScheduleOccurrence{Skipped: true}, state: "restore"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validateOccurrenceState(test.current, test.state)
			if !errors.Is(err, test.wantErr) {
				t.Fatalf("validateOccurrenceState() error = %v, want %v", err, test.wantErr)
			}
		})
	}
}

func TestOccurrenceStateCapabilityRemainsActionSpecific(t *testing.T) {
	tests := []struct {
		state string
		want  shared.Capability
	}{
		{state: "complete", want: shared.ScheduleUpdate()},
		{state: "reopen", want: shared.ScheduleUpdate()},
		{state: "skip", want: shared.OccurrenceUpdate()},
		{state: "restore", want: shared.OccurrenceUpdate()},
		{state: "delete", want: shared.ScheduleDelete()},
	}
	for _, test := range tests {
		got, err := occurrenceStateCapability(test.state)
		if err != nil || got != test.want {
			t.Fatalf("occurrenceStateCapability(%q) = (%+v, %v), want %+v", test.state, got, err, test.want)
		}
	}
}

func TestOccurrenceMutationUsesSeparateAssignmentPermission(t *testing.T) {
	assignee := "new-assignee"
	content := "new title"
	if got := occurrenceUpdateCapability(UpdateScheduleInput{AssigneeID: &assignee}); got != shared.ScheduleAssignmentUpdate() {
		t.Fatalf("assignee-only patch capability = %+v, want assignment update", got)
	}
	if got := occurrenceUpdateCapability(UpdateScheduleInput{Title: &content, AssigneeID: &assignee}); got != shared.ScheduleUpdate() {
		t.Fatalf("content patch capability = %+v, want schedule update", got)
	}
}

func TestVirtualOccurrenceMutationRequiresGeneratedIdentity(t *testing.T) {
	if _, err := virtualOccurrenceID(nil); !errors.Is(err, ErrOccurrenceIDGeneratorUnavailable) {
		t.Fatalf("virtualOccurrenceID(nil) error = %v, want unavailable", err)
	}
	for _, generated := range []string{"", " \t"} {
		if _, err := virtualOccurrenceID(func() string { return generated }); !errors.Is(err, ErrOccurrenceIDGeneratorUnavailable) {
			t.Errorf("virtualOccurrenceID(%q) error = %v, want unavailable", generated, err)
		}
	}

	ids := []string{"occurrence-1", "occurrence-2"}
	index := 0
	nextID := func() string {
		id := ids[index]
		index++
		return id
	}
	first, err := virtualOccurrenceID(nextID)
	if err != nil {
		t.Fatal(err)
	}
	second, err := virtualOccurrenceID(nextID)
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Fatalf("successive virtual occurrence IDs = %q and %q, want distinct IDs", first, second)
	}
}

type occurrenceReloadRuleRepo struct {
	RuleRepository
	revision dao.RuleRevision
}

func (r *occurrenceReloadRuleRepo) GetPersistedOccurrenceForMutation(context.Context, string, string, string, shared.Capability) (dao.ScheduleOccurrence, error) {
	return dao.ScheduleOccurrence{}, ErrOccurrenceNotFound
}
func (r *occurrenceReloadRuleRepo) GetEffectiveRevisionForDate(context.Context, string, string, string, shared.Capability) (dao.RuleRevision, error) {
	return r.revision, nil
}
func (*occurrenceReloadRuleRepo) ListPauseIntervals(context.Context, string, string) ([]dao.ProjectPauseInterval, error) {
	return nil, nil
}
func (*occurrenceReloadRuleRepo) ListOccurrenceOrigins(context.Context, string, []string, time.Time, time.Time) ([]dao.ScheduleOccurrenceOrigin, error) {
	return nil, nil
}
func (*occurrenceReloadRuleRepo) LockOccurrenceForMutation(context.Context, string, string, string, shared.Capability) error {
	return nil
}

type occurrenceReloadRepositories struct {
	rules *occurrenceReloadRuleRepo
}

func (r occurrenceReloadRepositories) ProjectLifecycle() shared.ProjectWorkLifecycle { return nil }
func (occurrenceReloadRepositories) Schedules() ScheduleRepository                   { return nil }
func (r occurrenceReloadRepositories) Rules() RuleRepository                         { return r.rules }

func TestLockedReloadRejectsAChangedVirtualSourceRevision(t *testing.T) {
	date := time.Now().UTC().AddDate(0, 0, 1).Format("2006-01-02")
	makeRevision := func(id string) dao.RuleRevision {
		return dao.RuleRevision{Rule: dao.Rule{
			ID: "rule", UserID: "owner", Revision: 1, EffectiveFrom: date, RecurrenceAnchorDate: date,
			LocalStart: date + "T09:00:00", LocalEnd: date + "T10:00:00", Timezone: "UTC",
			Frequency: "daily", Interval: 1, Title: "daily work", AssigneeID: "owner",
		}, RevisionID: id}
	}
	repo := &occurrenceReloadRuleRepo{revision: makeRevision("revision-before")}
	repositories := occurrenceReloadRepositories{rules: repo}
	initial, err := resolveOccurrenceForMutation(context.Background(), repo, "owner", "rule", date, shared.ScheduleUpdate(), false)
	if err != nil {
		t.Fatalf("resolve initial virtual occurrence: %v", err)
	}
	if initial.persisted || initial.value.SourceRevisionID != "revision-before" {
		t.Fatalf("unexpected initial projection: %+v", initial)
	}
	// A following rule edit changes the projected source while preserving
	// the virtual row's revision (zero), project, and assignee. The reload after
	// locking must still reject this stale mutation.
	repo.revision = makeRevision("revision-after")
	_, _, err = lockAndReloadOccurrence(context.Background(), repositories, "owner", "rule", date,
		initial, initial.value.Revision, shared.ScheduleUpdate(), nil, time.Now().UTC(), false)
	if !errors.Is(err, ErrOccurrenceRevisionMismatch) {
		t.Fatalf("locked reload error=%v, want ErrOccurrenceRevisionMismatch", err)
	}
}
