package store

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/betallsoph/shiftz/internal/ent"
	"github.com/betallsoph/shiftz/internal/ent/shop"
)

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

// ProvisionDashboardAccount sets the username and plan used by the owner dashboard.
func (r *ShopRepo) ProvisionDashboardAccount(ctx context.Context, shopID uuid.UUID, username, plan string) (*Shop, error) {
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

	row, err := r.client.Shop.UpdateOneID(shopID).
		SetDashboardUsername(norm).
		SetPlan(plan).
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
	return shopFromEnt(row), nil
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
