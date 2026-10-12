package dao

type User struct {
	ID                string
	FirstName         string
	LastName          string
	Email             string
	PasswordHash      string
	EmailVerifiedAt   int64
	AuthVersion       int64
	Timezone          string
	WakeUpTime        string
	Bedtime           string
	MinimumGapMinutes int
	CreatedAt         int64
	UpdatedAt         int64
}
