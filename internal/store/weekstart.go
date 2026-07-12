package store

import "time"

// WeekStart returns Monday 00:00:00 in loc for the calendar week containing t.
func WeekStart(t time.Time, loc *time.Location) time.Time {
	if loc == nil {
		loc = time.UTC
	}
	local := t.In(loc)
	daysSinceMonday := int(local.Weekday() - time.Monday)
	if daysSinceMonday < 0 {
		daysSinceMonday += 7
	}
	monday := local.AddDate(0, 0, -daysSinceMonday)
	return time.Date(monday.Year(), monday.Month(), monday.Day(), 0, 0, 0, 0, loc)
}
