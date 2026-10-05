package shared

type ID interface {
	Generate() string
}
