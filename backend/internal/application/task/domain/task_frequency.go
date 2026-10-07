package domain

import "github.com/Najah7/task2todaytodo/internal/application/shared/recurrence"

// Task frequency names remain as aliases for TodoItem domain callers. The
// frequency policy itself belongs to the shared recurrence package.
type TaskFrequency = recurrence.Frequency
type TaskFrequencies = recurrence.Frequencies

var (
	ErrTaskFrequencyEmpty   = recurrence.ErrFrequencyEmpty
	ErrTaskFrequencyInvalid = recurrence.ErrFrequencyInvalid
)

const (
	OnceIntervalWeeks       = recurrence.OnceIntervalWeeks
	WeeklyIntervalWeeks     = recurrence.WeeklyIntervalWeeks
	BiWeeklyIntervalWeeks   = recurrence.BiWeeklyIntervalWeeks
	FourWeekIntervalWeeks   = recurrence.FourWeekIntervalWeeks
	QuarterlyIntervalWeeks  = recurrence.QuarterlyIntervalWeeks
	SemiAnnualIntervalWeeks = recurrence.SemiAnnualIntervalWeeks
	AnnualIntervalWeeks     = recurrence.AnnualIntervalWeeks
)

func NewTaskFrequency(value string) (TaskFrequency, error) { return recurrence.NewFrequency(value) }
