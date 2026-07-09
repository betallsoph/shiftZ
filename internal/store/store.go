// Package store contains shiftbot's Postgres repositories, written with pgx
// and plain SQL (no ORM). Every table is multi-tenant: shop_id appears on
// every row and every query filters by it (or reaches rows through a
// shop-scoped join).
//
// This package must not import llm, telegram, solver or scheduler.
package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrNotFound is returned when a query matches no rows.
var ErrNotFound = errors.New("store: not found")

// Store bundles the connection pool and repositories.
type Store struct {
	Pool         *pgxpool.Pool
	Shops        *ShopRepo
	Employees    *EmployeeRepo
	Availability *AvailabilityRepo
	Votes        *VoteRepo
}

// New connects to Postgres and returns a ready Store.
func New(ctx context.Context, databaseURL string) (*Store, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("store: connect: %w", err)
	}
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("store: ping: %w", err)
	}
	return &Store{
		Pool:         pool,
		Shops:        &ShopRepo{pool: pool},
		Employees:    &EmployeeRepo{pool: pool},
		Availability: &AvailabilityRepo{pool: pool},
		Votes:        &VoteRepo{pool: pool},
	}, nil
}

// Close releases the connection pool.
func (s *Store) Close() { s.Pool.Close() }
