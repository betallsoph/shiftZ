package store

import (
	"context"
	"crypto/subtle"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/betallsoph/shiftz/internal/ent"
	"github.com/betallsoph/shiftz/internal/ent/shop"
)

// ProvisionedCredentials is returned once when an owner dashboard account is provisioned.
type ProvisionedCredentials struct {
	Shop       *Shop
	OwnerToken string
}

// ByDashboardUsername fetches a shop by normalized dashboard username.
func (r *ShopRepo) ByDashboardUsername(ctx context.Context, username string) (*Shop, error) {
	norm := NormalizeDashboardUsername(username)
	if norm == "" {
		return nil, ErrNotFound
	}
	row, err := r.client.Shop.Query().Where(shop.DashboardUsername(norm)).Only(ctx)
	if ent.IsNotFound(err) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("store: shop by dashboard username: %w", err)
	}
	return shopFromEnt(row), nil
}

// VerifyDashboardCredentials looks up by username and constant-time verifies the owner token.
func (r *ShopRepo) VerifyDashboardCredentials(ctx context.Context, username, token string) (*Shop, error) {
	norm := NormalizeDashboardUsername(username)
	if norm == "" || token == "" {
		return nil, ErrInvalidCredentials
	}
	row, err := r.client.Shop.Query().Where(shop.DashboardUsername(norm)).Only(ctx)
	if ent.IsNotFound(err) {
		return nil, ErrInvalidCredentials
	}
	if err != nil {
		return nil, fmt.Errorf("store: verify dashboard credentials: %w", err)
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

// ProvisionDashboardAccount sets username and plan, rotates owner token, returns plaintext once.
func (r *ShopRepo) ProvisionDashboardAccount(ctx context.Context, shopID uuid.UUID, username, plan string) (*ProvisionedCredentials, error) {
	if err := ValidateDashboardUsername(username); err != nil {
		return nil, err
	}
	if err := ValidatePlan(plan); err != nil {
		return nil, err
	}
	norm := NormalizeDashboardUsername(username)
	plan = strings.ToLower(strings.TrimSpace(plan))

	existing, err := r.client.Shop.Query().Where(shop.DashboardUsername(norm)).Only(ctx)
	if err == nil && existing.ID != shopID {
		return nil, ErrAlreadyExists
	}
	if err != nil && !ent.IsNotFound(err) {
		return nil, fmt.Errorf("store: provision check username: %w", err)
	}

	token, err := NewDashboardToken()
	if err != nil {
		return nil, err
	}
	hash := HashDashboardToken(token)

	row, err := r.client.Shop.UpdateOneID(shopID).
		SetDashboardUsername(norm).
		SetPlan(plan).
		SetDashboardTokenHash(hash).
		Save(ctx)
	if ent.IsNotFound(err) {
		return nil, ErrNotFound
	}
	if err != nil {
		if ent.IsConstraintError(err) {
			return nil, ErrAlreadyExists
		}
		return nil, fmt.Errorf("store: provision dashboard account: %w", err)
	}
	return &ProvisionedCredentials{
		Shop:       shopFromEnt(row),
		OwnerToken: token,
	}, nil
}

// UpdatePlan changes the shop plan without rotating the owner token.
func (r *ShopRepo) UpdatePlan(ctx context.Context, shopID uuid.UUID, plan string) (*Shop, error) {
	if err := ValidatePlan(plan); err != nil {
		return nil, err
	}
	plan = strings.ToLower(strings.TrimSpace(plan))
	row, err := r.client.Shop.UpdateOneID(shopID).SetPlan(plan).Save(ctx)
	if ent.IsNotFound(err) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("store: update plan: %w", err)
	}
	return shopFromEnt(row), nil
}

// RotateDashboardToken issues a new owner token and invalidates the previous one.
func (r *ShopRepo) RotateDashboardToken(ctx context.Context, shopID uuid.UUID) (*ProvisionedCredentials, error) {
	token, err := NewDashboardToken()
	if err != nil {
		return nil, err
	}
	hash := HashDashboardToken(token)
	row, err := r.client.Shop.UpdateOneID(shopID).SetDashboardTokenHash(hash).Save(ctx)
	if ent.IsNotFound(err) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("store: rotate dashboard token: %w", err)
	}
	return &ProvisionedCredentials{
		Shop:       shopFromEnt(row),
		OwnerToken: token,
	}, nil
}
