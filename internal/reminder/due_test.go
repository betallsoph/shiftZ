package reminder

import (
	"testing"
	"time"
)

func TestTargetWeekStart(t *testing.T) {
	loc := time.FixedZone("ICT", 7*3600)
	// Thursday 2026-07-16 → next Monday 2026-07-20
	thu := time.Date(2026, 7, 16, 15, 0, 0, 0, loc)
	got := targetWeekStart(thu, loc)
	want := time.Date(2026, 7, 20, 0, 0, 0, 0, loc)
	if !got.Equal(want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestIsDueWindow(t *testing.T) {
	loc := time.FixedZone("ICT", 7*3600)
	due := time.Date(2026, 7, 16, 10, 0, 0, 0, loc)

	if isDue(time.Date(2026, 7, 16, 9, 59, 0, 0, loc), due) {
		t.Fatal("should not be due before window")
	}
	if !isDue(time.Date(2026, 7, 16, 10, 5, 0, 0, loc), due) {
		t.Fatal("should be due inside window")
	}
	if !isDue(time.Date(2026, 7, 16, 23, 0, 0, 0, loc), due) {
		t.Fatal("should be due before window end")
	}
	if isDue(time.Date(2026, 7, 17, 10, 0, 0, 0, loc), due) {
		t.Fatal("should not be due after 24h window")
	}
}

func TestReminderDueAt(t *testing.T) {
	loc := time.FixedZone("ICT", 7*3600)
	weekStart := time.Date(2026, 7, 20, 0, 0, 0, 0, loc)
	got := reminderDueAt(weekStart, loc)
	want := time.Date(2026, 7, 16, 10, 0, 0, 0, loc)
	if !got.Equal(want) {
		t.Fatalf("reminder due = %v want %v", got, want)
	}
}

func TestNagDueAt(t *testing.T) {
	loc := time.FixedZone("ICT", 7*3600)
	weekStart := time.Date(2026, 7, 20, 0, 0, 0, 0, loc)
	got := nagDueAt(weekStart, loc)
	want := time.Date(2026, 7, 18, 10, 0, 0, 0, loc)
	if !got.Equal(want) {
		t.Fatalf("nag due = %v want %v", got, want)
	}
}
