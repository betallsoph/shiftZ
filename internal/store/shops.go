package store

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ShopRepo is the example repository for the shops table. All other
// repositories follow the same pattern: plain SQL, explicit scanning,
// shop_id scoping on every tenant-owned row.
type ShopRepo struct {
	pool *pgxpool.Pool
}

// Create inserts a new shop with a fresh invite code and returns it.
func (r *ShopRepo) Create(ctx context.Context, name, timezone string, ownerTelegramID int64) (*Shop, error) {
	code, err := newInviteCode()
	if err != nil {
		return nil, err
	}
	row := r.pool.QueryRow(ctx, `
		INSERT INTO shops (name, timezone, invite_code, owner_telegram_id)
		VALUES ($1, $2, $3, $4)
		RETURNING id, name, timezone, invite_code, owner_telegram_id, created_at`,
		name, timezone, code, ownerTelegramID)
	return scanShop(row)
}

// ByID fetches a shop by primary key.
func (r *ShopRepo) ByID(ctx context.Context, id int64) (*Shop, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, name, timezone, invite_code, owner_telegram_id, created_at
		FROM shops WHERE id = $1`, id)
	return scanShop(row)
}

// ByInviteCode fetches the shop an employee is joining.
func (r *ShopRepo) ByInviteCode(ctx context.Context, code string) (*Shop, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, name, timezone, invite_code, owner_telegram_id, created_at
		FROM shops WHERE invite_code = $1`, code)
	return scanShop(row)
}

func scanShop(row pgx.Row) (*Shop, error) {
	var s Shop
	err := row.Scan(&s.ID, &s.Name, &s.Timezone, &s.InviteCode, &s.OwnerTelegramID, &s.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("store: scan shop: %w", err)
	}
	return &s, nil
}

func newInviteCode() (string, error) {
	var b [6]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("store: invite code: %w", err)
	}
	return hex.EncodeToString(b[:]), nil
}
