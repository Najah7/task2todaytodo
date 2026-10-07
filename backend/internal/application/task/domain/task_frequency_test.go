package domain

import "testing"

func mustTaskFrequency(t *testing.T, value string) TaskFrequency {
	t.Helper()
	frequency, err := NewTaskFrequency(value)
	if err != nil {
		t.Fatal(err)
	}
	return frequency
}
