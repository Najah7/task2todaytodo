package usecase

import (
	"testing"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/schedule/domain"
)

func TestLastSafeBackfillDateRespectsSameDayInstantCutoff(t *testing.T) {
	day := time.Date(2025, time.October, 11, 0, 0, 0, 0, time.UTC)
	rule := domain.RecurrenceRule{
		Start:    time.Date(2025, time.October, 11, 9, 0, 0, 0, time.UTC),
		End:      time.Date(2025, time.October, 11, 10, 0, 0, 0, time.UTC),
		Timezone: "UTC", Frequency: "daily", Interval: 1,
	}
	tests := []struct {
		name       string
		cutoffHour int
		wantDay    int
	}{
		{name: "before today's start leaves today uncheckpointed", cutoffHour: 8, wantDay: 10},
		{name: "equal to today's start is exclusive", cutoffHour: 9, wantDay: 10},
		{name: "after today's start includes today", cutoffHour: 10, wantDay: 11},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cutoff := time.Date(2025, time.October, 11, test.cutoffHour, 0, 0, 0, time.UTC)
			got, err := lastSafeBackfillDate(rule, day, day, day, cutoff)
			if err != nil {
				t.Fatalf("lastSafeBackfillDate() error = %v", err)
			}
			if got.Day() != test.wantDay {
				t.Fatalf("lastSafeBackfillDate() = %s, want day %d", got.Format("2006-01-02"), test.wantDay)
			}
		})
	}
}
