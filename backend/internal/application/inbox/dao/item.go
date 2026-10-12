package dao

type ItemType string

const (
	ItemTypeTask     ItemType = "task"
	ItemTypeSchedule ItemType = "schedule"
)

type Item struct {
	Type        ItemType
	ID          string
	Title       string
	ProjectID   string
	ProjectName string
	CreatedAt   string
	Task        *TaskPayload
	Schedule    *SchedulePayload
}

type TaskPayload struct {
	TaskID                   string
	Revision                 int32
	Description              string
	AssigneeID               string
	DueDate                  int64
	RemainingDays            *int
	EstimatedMinutes         *int
	EstimateSource           string
	ActualMinutes            *int
	Progress                 int
	Priority                 string
	Status                   string
	ActionItemCount          int
	ActionItemCompletedCount int
	CanUpdate                bool
	CanAssign                bool
	Started                  bool
	EstimateEditable         bool
	CanAddActionItem         bool
	CanAddToToday            bool
}

type SchedulePayload struct {
	OccurrenceID       *string
	RuleID             string
	OccurrenceDate     string
	SourceRevisionID   string
	AssigneeID         string
	Description        string
	Location           string
	Timezone           string
	AllDay             bool
	Completed          bool
	Skipped            bool
	StartAt            int64
	EndAt              int64
	Revision           int32
	CanUpdate          bool
	CanAssign          bool
	CanUpdateFollowing bool
}
