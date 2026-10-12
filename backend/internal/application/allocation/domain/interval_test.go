package domain

import (
	"errors"
	"testing"
	"time"
)

func TestValidateNoOverlap(t *testing.T) {
	base := time.Date(2026, 10, 12, 9, 0, 0, 0, time.UTC)
	interval := func(start, end int) Interval {
		return Interval{Start: base.Add(time.Duration(start) * time.Minute), End: base.Add(time.Duration(end) * time.Minute)}
	}
	t.Run("adjacent intervals allowed", func(t *testing.T) {
		if err := ValidateNoOverlap([]Interval{interval(0, 30), interval(30, 60)}); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("overlap rejected", func(t *testing.T) {
		if err := ValidateNoOverlap([]Interval{interval(0, 31), interval(30, 60)}); !errors.Is(err, ErrIntervalOverlap) {
			t.Fatalf("got %v, want overlap error", err)
		}
	})
	t.Run("invalid interval rejected", func(t *testing.T) {
		if err := ValidateNoOverlap([]Interval{interval(20, 20)}); !errors.Is(err, ErrInvalidInterval) {
			t.Fatalf("got %v, want invalid interval error", err)
		}
	})
}
