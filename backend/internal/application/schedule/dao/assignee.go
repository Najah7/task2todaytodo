package dao

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
