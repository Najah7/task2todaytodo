package dao

type ProjectRevision struct {
	ID          string
	Revision    int32
	UserID      string
	Type        string
	Title       string
	Goal        string
	Description string
	Priority    string
	StartDate   *string
	EndDate     *string
	DeletedAt   *int64
	CreatedAt   int64
	UpdatedAt   int64
	ChangedBy   string
	ChangedAt   int64
	CursorAt    string
}

type TaskRevision struct {
	ID               string
	Revision         int32
	UserID           string
	ProjectID        string
	AssigneeID       string
	Title            string
	Description      string
	DueDate          int64
	EstimatedMinutes *int
	ActualMinutes    *int
	Priority         string
	Status           string
	DeletedAt        *int64
	CreatedAt        int64
	UpdatedAt        int64
	ChangedBy        string
	ChangedAt        int64
	CursorAt         string
}
