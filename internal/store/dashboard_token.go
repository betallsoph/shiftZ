package store

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"

	"github.com/google/uuid"

	"github.com/betallsoph/shiftz/internal/ent"
)

const dashboardTokenPrefix = "sz_owner_"

// NewDashboardToken returns a high-entropy owner dashboard token.
func NewDashboardToken() (string, error) {
	var b [24]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("store: dashboard token: %w", err)
	}
	return dashboardTokenPrefix + hex.EncodeToString(b[:]), nil
}

// HashDashboardToken returns the SHA-256 hex digest of a dashboard token.
func HashDashboardToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// SetDashboardTokenHash stores the hash for a shop's owner dashboard token.
func (r *ShopRepo) SetDashboardTokenHash(ctx context.Context, shopID uuid.UUID, hash string) error {
	if err := r.client.Shop.UpdateOneID(shopID).SetDashboardTokenHash(hash).Exec(ctx); err != nil {
		if ent.IsNotFound(err) {
			return ErrNotFound
		}
		return fmt.Errorf("store: set dashboard token hash: %w", err)
	}
	return nil
}

// VerifyDashboardToken checks the token for shopID and returns the shop on success.
func (r *ShopRepo) VerifyDashboardToken(ctx context.Context, shopID uuid.UUID, token string) (*Shop, error) {
	row, err := r.client.Shop.Get(ctx, shopID)
	if ent.IsNotFound(err) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("store: verify dashboard token: %w", err)
	}
	if row.DashboardTokenHash == nil || *row.DashboardTokenHash == "" {
		return nil, ErrInvalidCredentials
	}
	want := []byte(*row.DashboardTokenHash)
	got := []byte(HashDashboardToken(token))
	if subtle.ConstantTimeCompare(want, got) != 1 {
		return nil, ErrInvalidCredentials
	}
	return shopFromEnt(row), nil
}
