package dao

type Role struct {
	ID          string
	Name        string
	Permissions []Permission
}

type Permission struct {
	ID           int64
	ResourceID   string
	ResourceName string
	Action       string
	Effect       string
	Description  string
}
