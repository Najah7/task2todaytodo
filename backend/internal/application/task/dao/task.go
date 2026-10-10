package dao

type Task struct {
	ID                       string
	UserID                   string
	ProjectID                string
	AssigneeID               string
	Title                    string
	Description              string
	DueDate                  int64
	RemainingDays            *int
	ManualEstimatedMinutes   *int
	EstimatedMinutes         *int
	EstimateSource           string
	ActualMinutes            *int
	Progress                 int
	Priority                 Priority
	Status                   TaskStatus
	CreatedAt                int64
	UpdatedAt                int64
	Revision                 int32
	DeletedAt                *int64
	ChangedBy                string
	CursorCreatedAt          string
	ProjectName              string
	ProjectStatus            string
	ActionItemCount          int
	ActionItemCompletedCount int
	CanUpdate                bool
}

type TaskDetails struct {
	Task        Task
	ActionItems []ActionItem
}

type TaskProgressCounts struct {
	Total     int
	Completed int
}

type TaskEstimateSource struct {
	ManualEstimatedMinutes *int
	EstimatedMinutes       *int
	EstimateSource         string
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

type TaskListProjectionSources struct {
	ActionItemsByTask map[string][]ActionItem
	SkippedByTask     map[string]map[string]map[string]bool
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
