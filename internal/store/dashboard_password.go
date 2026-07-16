package store

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/betallsoph/shiftz/internal/ent"
	"github.com/betallsoph/shiftz/internal/ent/shop"
)

const (
	dashboardPasswordMinLen     = 6
	dashboardPasswordHintMaxLen = 200
	dashboardPasswordResetTTL   = time.Hour
	dashboardPasswordResetPrefix = "sz_pwreset_"
)

// ValidateDashboardPassword checks owner password strength.
func ValidateDashboardPassword(password string) error {
	if len(password) < dashboardPasswordMinLen {
		return fmt.Errorf("%w: dashboard password must be at least %d characters", ErrValidation, dashboardPasswordMinLen)
	}
	return nil
}

// NormalizeDashboardEmail lowercases and trims an owner dashboard email.
func NormalizeDashboardEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// ValidateDashboardEmail checks owner contact email format.
func ValidateDashboardEmail(email string) error {
	email = NormalizeDashboardEmail(email)
	if email == "" {
		return fmt.Errorf("%w: dashboard email is required", ErrValidation)
	}
	if len(email) > 254 || !strings.Contains(email, "@") || strings.HasPrefix(email, "@") || strings.HasSuffix(email, "@") {
		return fmt.Errorf("%w: invalid dashboard email", ErrValidation)
	}
	return nil
}

// ValidateDashboardPasswordHint checks optional owner password hint length.
func ValidateDashboardPasswordHint(hint string) error {
	if len(hint) > dashboardPasswordHintMaxLen {
		return fmt.Errorf("%w: dashboard password hint is too long", ErrValidation)
	}
	return nil
}

// HashDashboardPassword returns a bcrypt hash for an owner dashboard password.
func HashDashboardPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("store: hash dashboard password: %w", err)
	}
	return string(hash), nil
}

// NewDashboardPasswordResetToken returns a high-entropy one-time reset token.
func NewDashboardPasswordResetToken() (string, error) {
	var b [24]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("store: dashboard password reset token: %w", err)
	}
	return dashboardPasswordResetPrefix + hex.EncodeToString(b[:]), nil
}

// HasDashboardPassword reports whether shopID has a dashboard password configured.
func (r *ShopRepo) HasDashboardPassword(ctx context.Context, shopID uuid.UUID) (bool, error) {
	row, err := r.client.Shop.Get(ctx, shopID)
	if ent.IsNotFound(err) {
		return false, ErrNotFound
	}
	if err != nil {
		return false, fmt.Errorf("store: has dashboard password: %w", err)
	}
	return row.DashboardPasswordHash != nil && *row.DashboardPasswordHash != "", nil
}

// DashboardEmail returns the owner contact email on file, or empty when unset.
func (r *ShopRepo) DashboardEmail(ctx context.Context, shopID uuid.UUID) (string, error) {
	row, err := r.client.Shop.Get(ctx, shopID)
	if ent.IsNotFound(err) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("store: dashboard email: %w", err)
	}
	if row.DashboardEmail == nil {
		return "", nil
	}
	return *row.DashboardEmail, nil
}

// SetDashboardPassword stores the first owner dashboard password for shopID.
func (r *ShopRepo) SetDashboardPassword(ctx context.Context, shopID uuid.UUID, password string) error {
	if err := ValidateDashboardPassword(password); err != nil {
		return err
	}
	row, err := r.client.Shop.Get(ctx, shopID)
	if ent.IsNotFound(err) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("store: set dashboard password lookup: %w", err)
	}
	if row.DashboardPasswordHash != nil && *row.DashboardPasswordHash != "" {
		return ErrAlreadyExists
	}
	hash, err := HashDashboardPassword(password)
	if err != nil {
		return err
	}
	if err := r.client.Shop.UpdateOneID(shopID).SetDashboardPasswordHash(hash).Exec(ctx); err != nil {
		if ent.IsNotFound(err) {
			return ErrNotFound
		}
		return fmt.Errorf("store: set dashboard password: %w", err)
	}
	return nil
}

// SetDashboardCredentials stores the first owner password plus email and optional hint.
func (r *ShopRepo) SetDashboardCredentials(ctx context.Context, shopID uuid.UUID, password, email, hint string) error {
	if err := ValidateDashboardPassword(password); err != nil {
		return err
	}
	email = NormalizeDashboardEmail(email)
	if err := ValidateDashboardEmail(email); err != nil {
		return err
	}
	hint = strings.TrimSpace(hint)
	if err := ValidateDashboardPasswordHint(hint); err != nil {
		return err
	}
	row, err := r.client.Shop.Get(ctx, shopID)
	if ent.IsNotFound(err) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("store: set dashboard credentials lookup: %w", err)
	}
	if row.DashboardPasswordHash != nil && *row.DashboardPasswordHash != "" {
		return ErrAlreadyExists
	}
	hash, err := HashDashboardPassword(password)
	if err != nil {
		return err
	}
	upd := r.client.Shop.UpdateOneID(shopID).
		SetDashboardPasswordHash(hash).
		SetDashboardEmail(email)
	if hint != "" {
		upd.SetDashboardPasswordHint(hint)
	}
	if err := upd.Exec(ctx); err != nil {
		if ent.IsNotFound(err) {
			return ErrNotFound
		}
		return fmt.Errorf("store: set dashboard credentials: %w", err)
	}
	return nil
}

// VerifyDashboardPassword checks password for shopID.
func (r *ShopRepo) VerifyDashboardPassword(ctx context.Context, shopID uuid.UUID, password string) error {
	row, err := r.client.Shop.Get(ctx, shopID)
	if ent.IsNotFound(err) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("store: verify dashboard password: %w", err)
	}
	if row.DashboardPasswordHash == nil || *row.DashboardPasswordHash == "" {
		return ErrInvalidCredentials
	}
	if err := bcrypt.CompareHashAndPassword([]byte(*row.DashboardPasswordHash), []byte(password)); err != nil {
		return ErrInvalidCredentials
	}
	return nil
}

// IssueDashboardPasswordReset stores a one-time reset token and returns the plaintext token.
func (r *ShopRepo) IssueDashboardPasswordReset(ctx context.Context, shopID uuid.UUID) (string, error) {
	row, err := r.client.Shop.Get(ctx, shopID)
	if ent.IsNotFound(err) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("store: issue dashboard password reset lookup: %w", err)
	}
	if row.DashboardEmail == nil || strings.TrimSpace(*row.DashboardEmail) == "" {
		return "", ErrNotFound
	}
	token, err := NewDashboardPasswordResetToken()
	if err != nil {
		return "", err
	}
	expiresAt := time.Now().Add(dashboardPasswordResetTTL)
	hash := HashDashboardToken(token)
	if err := r.client.Shop.UpdateOneID(shopID).
		SetDashboardPasswordResetHash(hash).
		SetDashboardPasswordResetExpiresAt(expiresAt).
		Exec(ctx); err != nil {
		if ent.IsNotFound(err) {
			return "", ErrNotFound
		}
		return "", fmt.Errorf("store: issue dashboard password reset: %w", err)
	}
	return token, nil
}

// ResetDashboardPasswordWithToken replaces the password for the shop matching token.
func (r *ShopRepo) ResetDashboardPasswordWithToken(ctx context.Context, token, password string) (*Shop, error) {
	if err := ValidateDashboardPassword(password); err != nil {
		return nil, err
	}
	hash := HashDashboardToken(token)
	row, err := r.client.Shop.Query().
		Where(
			shop.DashboardPasswordResetHashEQ(hash),
			shop.DashboardPasswordResetExpiresAtGT(time.Now()),
		).
		Only(ctx)
	if ent.IsNotFound(err) {
		return nil, ErrInvalidCredentials
	}
	if err != nil {
		return nil, fmt.Errorf("store: reset dashboard password lookup: %w", err)
	}
	passwordHash, err := HashDashboardPassword(password)
	if err != nil {
		return nil, err
	}
	if err := r.client.Shop.UpdateOneID(row.ID).
		SetDashboardPasswordHash(passwordHash).
		ClearDashboardPasswordResetHash().
		ClearDashboardPasswordResetExpiresAt().
		Exec(ctx); err != nil {
		if ent.IsNotFound(err) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("store: reset dashboard password: %w", err)
	}
	updated, err := r.client.Shop.Get(ctx, row.ID)
	if err != nil {
		return nil, fmt.Errorf("store: reset dashboard password reload: %w", err)
	}
	return shopFromEnt(updated), nil
}
