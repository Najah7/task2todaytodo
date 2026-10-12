package dao

type User struct {
	ID         string
	FirstName  string
	LastName   string
	Email      string
	Password   string
	Timezone   string
	WakeUpTime string
	Bedtime    string
	CreatedAt  int64
	UpdatedAt  int64
}
