package usecase

type PatchField[T any] struct {
	Present bool
	Value   *T
}
