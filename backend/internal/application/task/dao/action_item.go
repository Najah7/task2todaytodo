package dao

type ActionItem struct {
	ID                  string
	TaskID              string
	Title               string
	Description         string
	DueDate             int64
	EstimatedMinutes    *int
	Priority            Priority
	Completed           bool
	Position            int
	IntervalWeeks       int
	Frequencies         []TaskFrequency
	RepeatState         string
	FrequencyAnchorDate int64
	SeriesID            string
	OccurrenceDate      string
	Timezone            string
	IsException         bool
	Deleted             bool
	CreatedAt           int64
	UpdatedAt           int64
}
