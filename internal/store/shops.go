package store

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"

	"github.com/google/uuid"

	"github.com/betallsoph/shiftz/internal/ent"
	"github.com/betallsoph/shiftz/internal/ent/shop"
)

// ShopRepo covers the shops table.
type ShopRepo struct {
	client *ent.Client
}

// Create inserts a new shop with a fresh invite code and returns it.
// telegramGroupID is the group chat the bot posts schedules and votes into.
func (r *ShopRepo) Create(ctx context.Context, name, timezone string, telegramGroupID int64) (*Shop, error) {
	code, err := newInviteCode()
	if err != nil {
		return nil, err
	}
	row, err := r.client.Shop.Create().
		SetName(name).
		SetTimezone(timezone).
		SetInviteCode(code).
		SetTelegramGroupID(telegramGroupID).
		Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("store: create shop: %w", err)
	}
	return shopFromEnt(row), nil
}

// ByID fetches a shop by primary key.
func (r *ShopRepo) ByID(ctx context.Context, id uuid.UUID) (*Shop, error) {
	row, err := r.client.Shop.Get(ctx, id)
	if ent.IsNotFound(err) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("store: shop by id: %w", err)
	}
	return shopFromEnt(row), nil
}

// ByInviteCode fetches the shop an employee is joining.
func (r *ShopRepo) ByInviteCode(ctx context.Context, code string) (*Shop, error) {
	row, err := r.client.Shop.Query().Where(shop.InviteCode(code)).Only(ctx)
	if ent.IsNotFound(err) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("store: shop by invite code: %w", err)
	}
	return shopFromEnt(row), nil
}

func newInviteCode() (string, error) {
	var b [6]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("store: invite code: %w", err)
	}
	return hex.EncodeToString(b[:]), nil
}
