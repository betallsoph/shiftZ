package store

import (
	"testing"
	"time"
)

func TestWeekStart(t *testing.T) {
	loc, err := time.LoadLocation("Asia/Ho_Chi_Minh")
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		in   time.Time
		want time.Time
	}{
		{
			name: "wednesday mid-week",
			in:   time.Date(2026, 7, 8, 15, 30, 0, 0, loc), // Wed
			want: time.Date(2026, 7, 6, 0, 0, 0, 0, loc),   // Mon
		},
		{
			name: "monday stays monday",
			in:   time.Date(2026, 7, 6, 12, 0, 0, 0, loc),
			want: time.Date(2026, 7, 6, 0, 0, 0, 0, loc),
		},
		{
			name: "sunday rolls back to monday",
			in:   time.Date(2026, 7, 12, 23, 59, 0, 0, loc), // Sun
			want: time.Date(2026, 7, 6, 0, 0, 0, 0, loc),    // Mon
		},
		{
			name: "utc nil location defaults to utc",
			in:   time.Date(2026, 7, 8, 10, 0, 0, 0, time.UTC),
			want: time.Date(2026, 7, 6, 0, 0, 0, 0, time.UTC),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got time.Time
			if tt.name == "utc nil location defaults to utc" {
				got = WeekStart(tt.in, nil)
			} else {
				got = WeekStart(tt.in, loc)
			}
			if !got.Equal(tt.want) {
				t.Fatalf("WeekStart() = %v, want %v", got, tt.want)
			}
		})
	}
}
