package dao

type ActionItem struct {
	ID               string
	TaskID           string
	Title            string
	Description      string
	DueDate          int64
	EstimatedMinutes *int
	Priority         Priority
	Completed        bool
	Position         int
	Deleted          bool
	CreatedAt        int64
	UpdatedAt        int64
}
