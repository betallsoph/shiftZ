package store

import (
	"database/sql"
	"time"
)

// Options configures the database/sql connection pool.
type Options struct {
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
}

// DefaultOptions returns Neon-friendly pool defaults for a small beta deployment.
func DefaultOptions() Options {
	return Options{
		MaxOpenConns:    5,
		MaxIdleConns:    2,
		ConnMaxLifetime: 30 * time.Minute,
		ConnMaxIdleTime: 5 * time.Minute,
	}
}

// Normalize clamps invalid values to safe defaults.
func (o Options) Normalize() Options {
	d := DefaultOptions()
	if o.MaxOpenConns <= 0 {
		o.MaxOpenConns = d.MaxOpenConns
	}
	if o.MaxIdleConns <= 0 {
		o.MaxIdleConns = d.MaxIdleConns
	}
	if o.MaxIdleConns > o.MaxOpenConns {
		o.MaxIdleConns = o.MaxOpenConns
	}
	if o.ConnMaxLifetime <= 0 {
		o.ConnMaxLifetime = d.ConnMaxLifetime
	}
	if o.ConnMaxIdleTime <= 0 {
		o.ConnMaxIdleTime = d.ConnMaxIdleTime
	}
	return o
}

func applyPool(db *sql.DB, opts Options) {
	opts = opts.Normalize()
	db.SetMaxOpenConns(opts.MaxOpenConns)
	db.SetMaxIdleConns(opts.MaxIdleConns)
	db.SetConnMaxLifetime(opts.ConnMaxLifetime)
	db.SetConnMaxIdleTime(opts.ConnMaxIdleTime)
}
