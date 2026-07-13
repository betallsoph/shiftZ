package store

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/betallsoph/shiftz/internal/ent"
	"github.com/betallsoph/shiftz/internal/ent/shop"
)

const telegramSetupPrefix = "tg_setup_"

// NewTelegramSetupCode returns a high-entropy Telegram group setup code.
func NewTelegramSetupCode() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("store: telegram setup code: %w", err)
	}
	return telegramSetupPrefix + hex.EncodeToString(b[:]), nil
}

// HashTelegramSetupCode returns the SHA-256 hex digest of a setup code.
func HashTelegramSetupCode(code string) string {
	sum := sha256.Sum256([]byte(code))
	return hex.EncodeToString(sum[:])
}

// RotateTelegramSetupCode stores a new hashed setup code for shopID.
func (r *ShopRepo) RotateTelegramSetupCode(ctx context.Context, shopID uuid.UUID, expiresAt time.Time) (string, error) {
	code, err := NewTelegramSetupCode()
	if err != nil {
		return "", err
	}
	hash := HashTelegramSetupCode(code)
	if err := r.client.Shop.UpdateOneID(shopID).
		SetTelegramSetupCodeHash(hash).
		SetTelegramSetupCodeExpiresAt(expiresAt).
		Exec(ctx); err != nil {
		if ent.IsNotFound(err) {
			return "", ErrNotFound
		}
		return "", fmt.Errorf("store: rotate telegram setup code: %w", err)
	}
	return code, nil
}

// VerifyTelegramSetupCode finds the shop for a setup code if it is still valid.
func (r *ShopRepo) VerifyTelegramSetupCode(ctx context.Context, code string, now time.Time) (*Shop, error) {
	hash := HashTelegramSetupCode(code)
	row, err := r.client.Shop.Query().
		Where(shop.TelegramSetupCodeHash(hash)).
		Only(ctx)
	if ent.IsNotFound(err) {
		return nil, ErrInvalidCredentials
	}
	if err != nil {
		return nil, fmt.Errorf("store: verify telegram setup code: %w", err)
	}
	if row.TelegramSetupCodeExpiresAt == nil || !row.TelegramSetupCodeExpiresAt.After(now) {
		return nil, ErrExpiredSetupCode
	}
	return shopFromEnt(row), nil
}

// SetTelegramGroup connects a shop to a Telegram group and clears setup codes.
func (r *ShopRepo) SetTelegramGroup(ctx context.Context, shopID uuid.UUID, groupID int64) error {
	if err := r.client.Shop.UpdateOneID(shopID).
		SetTelegramGroupID(groupID).
		ClearTelegramSetupCodeHash().
		ClearTelegramSetupCodeExpiresAt().
		Exec(ctx); err != nil {
		if ent.IsNotFound(err) {
			return ErrNotFound
		}
		return fmt.Errorf("store: set telegram group: %w", err)
	}
	return nil
}
