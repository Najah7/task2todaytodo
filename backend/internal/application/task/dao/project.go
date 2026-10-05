package dao

type Project struct {
	ID              string
	UserID          string
	Type            ProjectType
	Title           string
	Goal            string
	Description     string
	Progress        int
	Priority        Priority
	StartDate       *string
	EndDate         *string
	CreatedAt       int64
	UpdatedAt       int64
	Revision        int32
	DeletedAt       *int64
	ChangedBy       string
	CursorCreatedAt string
}

type ProjectDetails struct {
	Project Project
	Tasks   []Task
}

type ProjectType struct {
	Value     string
	Label     string
	LabelJp   string
	CreatedAt int64
	UpdatedAt int64
}
