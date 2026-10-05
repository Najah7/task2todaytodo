package dao

type ProjectMember struct {
	ProjectID string
	UserID    string
	RoleID    string
	RoleName  string
	FirstName string
	LastName  string
	Email     string
	AddedBy   string
	CreatedAt int64
	UpdatedAt int64
}

type TaskAssignee struct {
	ID           string
	FirstName    string
	LastName     string
	Email        string
	ProjectOwner bool
}
