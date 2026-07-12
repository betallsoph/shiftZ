// Package store contains shiftbot's data layer, implemented on the ent
// client (entgo.io) over Postgres. Every entity is multi-tenant: shop_id
// appears on every row and every query filters by it (or reaches rows
// through a shop-scoped lookup).
//
// The repositories wrap the ent client behind plain interfaces so solver,
// telegram and llm code never imports ent directly — the dependency
// direction stays clean and fakes are easy to write in tests.
// This package must not import llm, telegram, solver or scheduler.
package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	_ "github.com/jackc/pgx/v5/stdlib" // database/sql driver ent runs on

	"github.com/betallsoph/shiftz/internal/ent"
	// Registers schema defaults and hooks (required because the Shift
	// schema declares hooks; without this import defaults are nil).
	_ "github.com/betallsoph/shiftz/internal/ent/runtime"
)

// ErrNotFound is returned when a query matches no rows.
var ErrNotFound = errors.New("store: not found")

// ErrAlreadyExists is returned when schedules already exist for a shop week.
var ErrAlreadyExists = errors.New("store: schedules already exist for shop week")

// Store bundles the ent client and repositories.
type Store struct {
	// Client is the raw ent client, exposed for code that outgrows the
	// repositories (seeding, future schedule persistence).
	Client *ent.Client

	Shops        *ShopRepo
	Employees    *EmployeeRepo
	Shifts       *ShiftRepo
	Availability *AvailabilityRepo
	Schedules    *ScheduleRepo
	Rules        *RuleRepo
	Votes        *VoteRepo

	db *sql.DB
}

// New connects to Postgres via pgx's database/sql adapter and returns a
// ready Store. With debug true, every generated SQL statement is logged
// (dev only — it's verbose and logs parameters).
func New(ctx context.Context, databaseURL string, debug bool) (*Store, error) {
	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return nil, fmt.Errorf("store: open: %w", err)
	}
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := db.PingContext(pingCtx); err != nil {
		db.Close()
		return nil, fmt.Errorf("store: ping: %w", err)
	}
	client := ent.NewClient(ent.Driver(entsql.OpenDB(dialect.Postgres, db)))
	if debug {
		client = client.Debug()
	}
	return &Store{
		Client:       client,
		Shops:        &ShopRepo{client: client},
		Employees:    &EmployeeRepo{client: client},
		Shifts:       &ShiftRepo{client: client},
		Availability: &AvailabilityRepo{client: client},
		Schedules:    &ScheduleRepo{client: client},
		Rules:        &RuleRepo{client: client},
		Votes:        &VoteRepo{client: client},
		db:           db,
	}, nil
}

// NewWithClient returns a Store wired to an existing ent client. Useful for
// tests and tools that manage their own database connection.
func NewWithClient(client *ent.Client) *Store {
	return &Store{
		Client:       client,
		Shops:        &ShopRepo{client: client},
		Employees:    &EmployeeRepo{client: client},
		Shifts:       &ShiftRepo{client: client},
		Availability: &AvailabilityRepo{client: client},
		Schedules:    &ScheduleRepo{client: client},
		Rules:        &RuleRepo{client: client},
		Votes:        &VoteRepo{client: client},
	}
}

// Ping checks database reachability (used by health endpoints).
func (s *Store) Ping(ctx context.Context) error { return s.db.PingContext(ctx) }

// Close releases the underlying connections.
func (s *Store) Close() error { return s.Client.Close() }
