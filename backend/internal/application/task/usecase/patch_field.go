package usecase

// PatchField distinguishes an omitted JSON field from an explicit null.
// Present=false leaves the stored value unchanged; Present=true and Value=nil
// clears a nullable field or is rejected for a required field.
type PatchField[T any] struct {
	Present bool
	Value   *T
}
