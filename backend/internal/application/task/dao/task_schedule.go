package dao

type TaskSchedule struct {
	ID                  string
	TaskID              string
	Title               string
	Description         string
	Location            string
	IntervalWeeks       int
	Frequencies         []TaskFrequency
	RepeatState         string
	FrequencyAnchorDate int64
	StartAt             int64
	EndAt               int64
	SeriesID            string
	OccurrenceDate      string
	Timezone            string
	IsException         bool
	Completed           bool
	Deleted             bool
	CreatedAt           int64
	UpdatedAt           int64
	CursorStartAt       string
}
