package dao

// TaskTag is the Task-owned projection of an assigned shared Tag.
type TaskTag struct {
	ID        string
	UserID    string
	Name      string
	CreatedAt int64
	UpdatedAt int64
}
