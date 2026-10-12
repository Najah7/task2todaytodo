package usecase

import (
	"testing"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/allocation/dao"
)

func TestCollectionRevisionsAreStableAndScoped(t *testing.T) {
	first := dao.Rule{ID: "rule-b", ProjectID: "project-b", ProjectTitle: "B", Revision: 2, SourceRevisionID: "source-2", Weekday: 1, StartTime: "09:00", EndTime: "10:00", Timezone: "UTC", Frequency: "weekly", EffectiveFrom: time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC)}
	second := dao.Rule{ID: "rule-a", ProjectID: "project-a", ProjectTitle: "A", Revision: 1, SourceRevisionID: "source-1", Weekday: 2, StartTime: "11:00", EndTime: "12:00", Timezone: "UTC", Frequency: "weekly"}
	left := rulesRevision("owner", "UTC", []dao.Rule{first, second})
	right := rulesRevision("owner", "UTC", []dao.Rule{second, first})
	if left != right {
		t.Fatalf("revisions differ after reordering: %q != %q", left, right)
	}
	if left == rulesRevision("other-owner", "UTC", []dao.Rule{first, second}) {
		t.Fatal("revision did not include owner discriminator")
	}
	if left == rulesRevision("owner", "Asia/Tokyo", []dao.Rule{first, second}) {
		t.Fatal("revision did not include user timezone")
	}
	changed := first
	changed.SourceRevisionID = "source-3"
	if left == rulesRevision("owner", "UTC", []dao.Rule{changed, second}) {
		t.Fatal("revision did not include current rule identity")
	}

	plans := []dao.PlanItem{{ProjectID: "project-a", Title: "A", TargetMinutes: 0}}
	planRevision := plansRevision("owner", plans)
	if planRevision != plansRevision("owner", append([]dao.PlanItem(nil), plans...)) {
		t.Fatal("plan revision changed for equal persisted content")
	}
	if planRevision == plansRevision("owner", nil) {
		t.Fatal("plan revision did not change when explicit row was cleared")
	}
	if planRevision == plansRevision("other-owner", plans) {
		t.Fatal("plan revision did not include owner discriminator")
	}
}

func TestCollectionRevisionsUseIndependentResources(t *testing.T) {
	if rulesRevision("owner", "UTC", nil) == plansRevision("owner", nil) {
		t.Fatal("empty resource revisions collided")
	}
}
