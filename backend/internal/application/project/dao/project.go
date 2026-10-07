package dao

type ProjectProgressSources struct {
	Tasks     []ProjectTaskProgress
	Schedules []ProjectScheduleProgress
}

type ProjectTaskProgress struct {
	ProjectID string
	TaskID    string
	Done      bool
	Total     int
	Completed int
	Roots     []ProjectProgressRecurrence
}

type ProjectScheduleProgress struct {
	ProjectID string
	Total     int
	Completed int
	Roots     []ProjectProgressRecurrence
}

// ProjectProgressRecurrence contains recurrence facts read from storage. It
// deliberately carries no progress or eligibility result.
type ProjectProgressRecurrence struct {
	OccurrenceDate       string
	Timezone             string
	IntervalWeeks        int
	FrequencyAnchorDate  string
	Frequencies          []string
	OccurrenceSavedToday bool
	StartAt              string
	EndAt                string
}

type Project struct {
	ID, UserID               string
	Type                     ProjectType
	Title, Goal, Description string
	Progress                 int
	Priority                 Priority
	StartDate, EndDate       *string
	CreatedAt, UpdatedAt     int64
	Revision                 int32
	DeletedAt                *int64
	ChangedBy                string
	CursorCreatedAt          string
}

type ProjectType struct {
	Value, Label, LabelJp string
	CreatedAt, UpdatedAt  int64
}
type Priority struct {
	Value, Label, LabelJp string
	Weight                int
}
type ProjectMember struct {
	ProjectID, UserID, RoleID, RoleName, FirstName, LastName, Email, AddedBy string
	CreatedAt, UpdatedAt                                                     int64
}
type ProjectRevision struct {
	ID                                               string
	Revision                                         int32
	UserID, Type, Title, Goal, Description, Priority string
	StartDate, EndDate                               *string
	DeletedAt                                        *int64
	CreatedAt, UpdatedAt                             int64
	ChangedBy                                        string
	ChangedAt                                        int64
	CursorAt                                         string
}
