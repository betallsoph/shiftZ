// Package store contains shiftbot's data layer, implemented on the ent
// client (entgo.io) over Postgres. Every entity is multi-tenant: shop_id
// appears on every row and every query filters by it (or reaches rows
// through a shop-scoped lookup).
//
// The repository interfaces predate ent and are kept stable so telegram,
// scheduler and cmd code is independent of the persistence library.
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
)

// ErrNotFound is returned when a query matches no rows.
var ErrNotFound = errors.New("store: not found")

// Store bundles the ent client and repositories.
type Store struct {
	// Client is the raw ent client, exposed for code that outgrows the
	// repositories (seeding, future schedule persistence).
	Client *ent.Client

	Shops        *ShopRepo
	Employees    *EmployeeRepo
	Availability *AvailabilityRepo
	Votes        *VoteRepo

	db *sql.DB
}

// New connects to Postgres via pgx's database/sql adapter and returns a
// ready Store.
func New(ctx context.Context, databaseURL string) (*Store, error) {
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
	return &Store{
		Client:       client,
		Shops:        &ShopRepo{client: client},
		Employees:    &EmployeeRepo{client: client},
		Availability: &AvailabilityRepo{client: client},
		Votes:        &VoteRepo{client: client},
		db:           db,
	}, nil
}

// Ping checks database reachability (used by health endpoints).
func (s *Store) Ping(ctx context.Context) error { return s.db.PingContext(ctx) }

// Close releases the underlying connections.
func (s *Store) Close() error { return s.Client.Close() }
