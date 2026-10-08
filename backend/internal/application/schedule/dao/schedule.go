package dao

type Schedule struct {
	ID                  string
	UserID              string
	ProjectID           string
	ProjectDone         bool
	AssigneeID          string
	Title               string
	Description         string
	Location            string
	IntervalWeeks       int
	Frequencies         []Frequency
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
	Revision            int32
	ChangedBy           string
	CursorStartAt       string
}

type Frequency struct {
	Value   string
	Label   string
	LabelJp string
}

type Assignee struct {
	ID        string
	FirstName string
	LastName  string
	Email     string
	IsOwner   bool
}

type ScheduleTag struct {
	ID        string
	UserID    string
	Name      string
	CreatedAt int64
	UpdatedAt int64
}

type ScheduleRevision struct {
	ID                  string
	Revision            int32
	UserID              string
	ProjectID           string
	AssigneeID          string
	Title               string
	Description         string
	Location            string
	StartAt             int64
	EndAt               int64
	SeriesID            string
	OccurrenceDate      string
	Timezone            string
	IsException         bool
	Completed           bool
	DeletedAt           *int64
	RepeatState         string
	FrequencyAnchorDate *string
	IntervalWeeks       int
	Frequencies         []Frequency
	CreatedAt           int64
	UpdatedAt           int64
	ChangedBy           string
	ChangedAt           int64
	CursorAt            string
}
