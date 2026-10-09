package dao

type Task struct {
	ID               string
	UserID           string
	ProjectID        string
	AssigneeID       string
	Title            string
	Description      string
	DueDate          int64
	EstimatedMinutes *int
	ActualMinutes    *int
	Progress         int
	Priority         Priority
	Status           TaskStatus
	CreatedAt        int64
	UpdatedAt        int64
	Revision         int32
	DeletedAt        *int64
	ChangedBy        string
	CursorCreatedAt  string
}

type TaskDetails struct {
	Task        Task
	ActionItems []ActionItem
}

type TaskProgressCounts struct {
	Total     int
	Completed int
}

type ProgressRecurrence struct {
	TaskID               string
	SeriesID             string
	OccurrenceDate       string
	Timezone             string
	RepeatState          string
	FrequencyAnchorDate  int64
	IntervalWeeks        int
	Frequencies          []TaskFrequency
	StartAt              int64
	EndAt                int64
	OccurrenceSavedToday bool
}

type TaskProgressSources struct {
	Counts          map[string]TaskProgressCounts
	Statuses        map[string]TaskStatus
	ActionItemRoots []ProgressRecurrence
}

type TaskFrequency struct {
	Value     string
	Label     string
	LabelJp   string
	CreatedAt int64
	UpdatedAt int64
}

type Priority struct {
	Value     string
	Label     string
	LabelJp   string
	Weight    int
	CreatedAt int64
	UpdatedAt int64
}

type TaskStatus struct {
	Value     string
	Label     string
	LabelJp   string
	CreatedAt int64
	UpdatedAt int64
}
