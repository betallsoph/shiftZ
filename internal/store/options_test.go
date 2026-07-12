package store

import (
	"testing"
	"time"
)

func TestOptionsNormalizeDefaults(t *testing.T) {
	got := (Options{}).Normalize()
	want := DefaultOptions()
	if got != want {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestOptionsNormalizeClampsIdleAboveOpen(t *testing.T) {
	got := Options{MaxOpenConns: 3, MaxIdleConns: 10}.Normalize()
	if got.MaxOpenConns != 3 {
		t.Fatalf("MaxOpenConns = %d", got.MaxOpenConns)
	}
	if got.MaxIdleConns != 3 {
		t.Fatalf("MaxIdleConns = %d, want 3", got.MaxIdleConns)
	}
}

func TestOptionsNormalizePreservesValidOverrides(t *testing.T) {
	got := Options{
		MaxOpenConns:    8,
		MaxIdleConns:    4,
		ConnMaxLifetime: 10 * time.Minute,
		ConnMaxIdleTime: 2 * time.Minute,
	}.Normalize()
	if got.MaxOpenConns != 8 || got.MaxIdleConns != 4 {
		t.Fatalf("got %+v", got)
	}
}
