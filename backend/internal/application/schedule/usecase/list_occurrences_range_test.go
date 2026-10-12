package usecase

import (
	"context"
	"testing"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/schedule/dao"
)

type rangeProjectionRuleRepo struct {
	RuleRepository
	revision dao.RuleRevision
	calls    int
}

func (r *rangeProjectionRuleRepo) ListRuleRevisionsForRange(context.Context, string, time.Time, time.Time) ([]dao.RuleRevision, error) {
	r.calls++
	return []dao.RuleRevision{r.revision}, nil
}
func (*rangeProjectionRuleRepo) ListPersistedOccurrences(context.Context, string, time.Time, time.Time, bool) ([]dao.ScheduleOccurrence, error) {
	return nil, nil
}
func (*rangeProjectionRuleRepo) ListOccurrenceOrigins(context.Context, string, []string, time.Time, time.Time) ([]dao.ScheduleOccurrenceOrigin, error) {
	return nil, nil
}
func (*rangeProjectionRuleRepo) ListPauseIntervals(context.Context, string, string) ([]dao.ProjectPauseInterval, error) {
	return nil, nil
}

type rangeProjectionRepos struct {
	Repositories
	rules RuleRepository
}

func (r rangeProjectionRepos) Rules() RuleRepository { return r.rules }

func TestRangeInTxProjectsVirtualOccurrencesUsingCallerRepositories(t *testing.T) {
	repo := &rangeProjectionRuleRepo{revision: dao.RuleRevision{Rule: dao.Rule{
		ID: "rule", UserID: "owner", Revision: 1, EffectiveFrom: "2026-10-12",
		RecurrenceAnchorDate: "2026-10-12", LocalStart: "2026-10-12 09:00:00",
		LocalEnd: "2026-10-12 10:00:00", Timezone: "Asia/Tokyo", Frequency: "once",
		Title: "review", AssigneeID: "owner",
	}, RevisionID: "revision"}}
	from := time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC)
	to := from.Add(24 * time.Hour)
	rows, err := (&ListOccurrencesUseCase{}).RangeInTx(context.Background(), "owner", from, to, rangeProjectionRepos{rules: repo})
	if err != nil {
		t.Fatalf("RangeInTx() error = %v", err)
	}
	if repo.calls != 1 || len(rows) != 1 {
		t.Fatalf("RangeInTx() calls=%d rows=%+v; want one owner read and one projected row", repo.calls, rows)
	}
	if rows[0].RuleID != "rule" || rows[0].OccurrenceDate != "2026-10-12" || rows[0].AssigneeID != "owner" {
		t.Fatalf("projected occurrence = %+v", rows[0])
	}
}
