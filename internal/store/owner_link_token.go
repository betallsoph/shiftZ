package store

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/betallsoph/shiftz/internal/ent"
	"github.com/betallsoph/shiftz/internal/ent/shop"
)

const (
	ownerLinkTokenTTL    = 15 * time.Minute
	ownerLinkTokenPrefix = "sz_ownerlink_"
)

// NewOwnerLinkToken returns a high-entropy one-time owner Telegram link token.
func NewOwnerLinkToken() (string, error) {
	var b [24]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("store: owner link token: %w", err)
	}
	return ownerLinkTokenPrefix + hex.EncodeToString(b[:]), nil
}

// IssueOwnerLinkToken stores a one-time link token and returns the plaintext token.
func (r *ShopRepo) IssueOwnerLinkToken(ctx context.Context, shopID uuid.UUID) (string, error) {
	if _, err := r.client.Shop.Get(ctx, shopID); ent.IsNotFound(err) {
		return "", ErrNotFound
	} else if err != nil {
		return "", fmt.Errorf("store: issue owner link token lookup: %w", err)
	}
	token, err := NewOwnerLinkToken()
	if err != nil {
		return "", err
	}
	expiresAt := time.Now().Add(ownerLinkTokenTTL)
	hash := HashDashboardToken(token)
	if err := r.client.Shop.UpdateOneID(shopID).
		SetOwnerLinkTokenHash(hash).
		SetOwnerLinkTokenExpiresAt(expiresAt).
		Exec(ctx); err != nil {
		if ent.IsNotFound(err) {
			return "", ErrNotFound
		}
		return "", fmt.Errorf("store: issue owner link token: %w", err)
	}
	return token, nil
}

// ConsumeOwnerLinkToken validates a one-time link token and returns the matching shop.
// The token is cleared on success so it cannot be reused.
func (r *ShopRepo) ConsumeOwnerLinkToken(ctx context.Context, token string) (*Shop, error) {
	hash := HashDashboardToken(token)
	row, err := r.client.Shop.Query().
		Where(
			shop.OwnerLinkTokenHashEQ(hash),
			shop.OwnerLinkTokenExpiresAtGT(time.Now()),
		).
		Only(ctx)
	if ent.IsNotFound(err) {
		return nil, ErrInvalidCredentials
	}
	if err != nil {
		return nil, fmt.Errorf("store: consume owner link token lookup: %w", err)
	}
	if err := r.client.Shop.UpdateOneID(row.ID).
		ClearOwnerLinkTokenHash().
		ClearOwnerLinkTokenExpiresAt().
		Exec(ctx); err != nil {
		if ent.IsNotFound(err) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("store: consume owner link token clear: %w", err)
	}
	return shopFromEnt(row), nil
}
