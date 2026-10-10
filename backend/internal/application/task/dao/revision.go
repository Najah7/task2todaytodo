package dao

type TaskRevision struct {
	ID                     string
	Revision               int32
	UserID                 string
	ProjectID              string
	AssigneeID             string
	Title                  string
	Description            string
	DueDate                int64
	ManualEstimatedMinutes *int
	ActualMinutes          *int
	Priority               string
	Status                 string
	DeletedAt              *int64
	CreatedAt              int64
	UpdatedAt              int64
	ChangedBy              string
	ChangedAt              int64
	CursorAt               string
}
